package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	pb "github.com/minorhacks/bazel_remote_query/proto"

	"cloud.google.com/go/storage"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/golang/glog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	configPath = flag.String("config", "", "Path to textproto WorkerConfig")
)

type Worker struct {
	workspaceMap map[string]*Workspace
	gcsBucket    *storage.BucketHandle
}

func (w *Worker) HandleJob(ctx context.Context, job *pb.QueryJob) (string, error) {
	repo := job.GetSource().GetRepo()
	workspace, ok := w.workspaceMap[repo]
	if !ok {
		return "", fmt.Errorf("workspace for repo %q not found", repo)
	}

	ref := job.GetSource().GetCommittish()
	if err := workspace.repo.FetchContext(context.TODO(), &git.FetchOptions{
		RefSpecs: []gitconfig.RefSpec{gitconfig.RefSpec(fmt.Sprintf("+%s:%s", ref, ref))},
		Force:    true,
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return "", fmt.Errorf("failed to fetch ref %q: %w", ref, err)
	}

	wt, err := workspace.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree for %q: %w", repo, err)
	}
	if err := wt.Checkout(&git.CheckoutOptions{
		Hash:  plumbing.NewHash(ref), // TODO: doesn't work for branch names, etc.
		Force: true,
	}); err != nil {
		return "", fmt.Errorf("failed to checkout ref %q: %w", ref, err)
	}
	glog.V(1).Infof("Checkout successful")

	// Run query in bazel workspace
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	res, err := workspace.Query(ctx, job.GetQuery())
	if err != nil {
		return "", err
	}
	glog.V(1).Infof("Query successful")

	// If success, upload result to GCS
	objName := fmt.Sprintf("%s.pb", job.GetId())
	obj := w.gcsBucket.Object(objName)
	objWriter := obj.NewWriter(ctx)
	if _, err := io.Copy(objWriter, res); err != nil {
		return "", fmt.Errorf("failed to copy results to GCS: %w", err)
	}
	if err := objWriter.Close(); err != nil {
		return "", fmt.Errorf("failed to flush results to GCS: %w", err)
	}
	glog.V(1).Infof("Upload successful")
	return fmt.Sprintf("gs://%s/%s", obj.BucketName(), obj.ObjectName()), nil

}

type Workspace struct {
	path string
	repo *git.Repository
}

func (w *Workspace) Query(ctx context.Context, query string) (res io.ReadCloser, err error) {
	cmd := exec.CommandContext(ctx, "bazel", "query", query, "--output=proto")
	stdout, err := os.CreateTemp("", "bazel_remote_query_*.pb")
	if err != nil {
		return nil, fmt.Errorf("failed to create query output file: %w", err)
	}
	defer func() {
		if err != nil {
			glog.V(1).Infof("Query failed in %q; deleting output file", w.path)
			logIfErr("closing output file", res.Close())
			logIfErr("deleting output file", os.Remove(res.(*os.File).Name()))
			res = nil
		}
	}()
	var stderr bytes.Buffer
	cmd.Dir = w.path
	cmd.Stdout = stdout
	cmd.Stderr = &stderr
	glog.V(1).Infof("Running query %q in %q to output %q...", query, w.path, stdout.Name())
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("bazel query failed: %v\nStderr: %s", err, stderr.String())
	}
	if _, err := stdout.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to reset file offset in %q: %w", stdout.Name(), err)
	}

	glog.V(1).Infof("Query output successfully saved in %q", stdout.Name())
	return stdout, nil
}

func main() {
	flag.Parse()

	config, err := loadConfig(*configPath)
	exitIf(err)

	// TODO: Parameterize tiem
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Minute)
	worker, err := New(ctx, config)
	exitIf(err)

	conn, err := grpc.Dial(
		config.GetDispatcherAddress(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	exitIf(err)
	defer conn.Close()
	client := pb.NewQueryDispatchClient(conn)

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute) // TODO: parameterize
		defer cancel()
		job, err := client.GetQueryJob(ctx, &pb.GetQueryJobRequest{})
		st, ok := status.FromError(err)
		if !ok {
			glog.Errorf("Failed to get next query job: %v", err)
			time.Sleep(5 * time.Second) // TODO: parameterize
			continue
		} else if st.Code() != codes.OK {
			glog.Errorf("Failed to get next query job: %v", st.Message())
			time.Sleep(5 * time.Second) // TODO: parameterize
			continue
		}

		nextPoll := job.GetNextPollTime().AsTime()
		if j := job.GetJob(); j != nil {
			url, err := worker.HandleJob(ctx, j)
			req := &pb.FinishQueryJobRequest{
				QueryJobId: j.GetId(),
			}
			if err != nil {
				req.Result = &pb.FinishQueryJobRequest_FailureMessage{
					FailureMessage: err.Error(),
				}
			} else {
				req.Result = &pb.FinishQueryJobRequest_QueryResultGcsLocation{
					QueryResultGcsLocation: url,
				}
			}
			_, err = client.FinishQueryJob(ctx, req)
			logIfErr("sending FinishQuery request", err)
			glog.Infof("Finished processing job")
		} else {
			glog.Infof("No pending queries; sleeping until %s", nextPoll.String())
		}
		time.Sleep(time.Until(nextPoll))
	}
}

func exitIf(err error) {
	if err != nil {
		glog.Exit(err)
	}
}

func logIfErr(while string, err error) {
	if err != nil {
		glog.Errorf("Error encountered while %s: %v", while, err)
	}
}

func loadConfig(path string) (*pb.WorkerConfig, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config %q: %w", path, err)
	}
	var config pb.WorkerConfig
	if err := prototext.Unmarshal(contents, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config %q: %w", path, err)
	}
	return &config, nil
}

func New(ctx context.Context, config *pb.WorkerConfig) (*Worker, error) {
	workspaceMap := map[string]*Workspace{}
	for _, repo := range config.GetGitRepositoryUrls() {
		// If the target dir already exists, remove it
		targetDir := filepath.Join(config.GetBaseDir(), filepath.Base(repo))
		r, err := git.PlainOpen(targetDir)
		if err != nil && errors.Is(err, git.ErrRepositoryNotExists) {
			// Clone into target dir
			glog.Infof("Cloning %s into %s...", repo, targetDir)
			r, err = git.PlainCloneContext(ctx, targetDir, false, &git.CloneOptions{
				URL: repo,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to clone repo %q: %w", repo, err)
			}
			glog.Infof("Successfully cloned %s into %s", repo, targetDir)
		} else if err != nil {
			return nil, fmt.Errorf("failed to open repository in %q: %w", targetDir, err)
		} else {
			glog.Infof("Opened existing repository for %s at %s", repo, targetDir)
		}
		workspaceMap[repo] = &Workspace{repo: r, path: targetDir}
	}
	gcsClient, err := storage.NewClient(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}
	return &Worker{
		workspaceMap: workspaceMap,
		gcsBucket:    gcsClient.Bucket(config.GetResultsGcsBucket()),
	}, nil
}
