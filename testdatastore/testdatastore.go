package testdatastore

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-yaml/yaml"
)

type Datastore struct {
	cmd       *exec.Cmd
	ctxCancel func()
	newEnv    map[string]string
	oldEnv    map[string]string
}

func (d *Datastore) Close() error {
	d.ctxCancel()
	err := d.cmd.Wait()
	popEnv(d.newEnv, d.oldEnv)
	return err
}

func startEmulator(ctx context.Context, dataDir string, writeToDisk bool) (*exec.Cmd, func(), error) {
	ctx, ctxCancel := context.WithCancel(ctx)
	args := []string{
		"gcloud",
		"beta",
		"emulators",
		"datastore",
		"start",
		"--data-dir=" + dataDir,
	}
	if !writeToDisk {
		args = append(args, "--no-store-on-disk")
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start datastore emulator: %w", err)
	}
	time.Sleep(3 * time.Second)
	if !strings.Contains(stderr.String(), "Dev App Server is now running.") {
		ctxCancel()
		return nil, nil, fmt.Errorf("canary output not found while starting datastore emulator")
	}
	return cmd, ctxCancel, nil
}

func pushEnv(newVars map[string]string) map[string]string {
	oldVars := map[string]string{}

	for k, v := range newVars {
		if old, ok := os.LookupEnv(k); ok {
			oldVars[k] = old
		}
		os.Setenv(k, v)
	}
	return oldVars
}

func popEnv(newVars map[string]string, oldVars map[string]string) {
	for k := range newVars {
		if old, ok := oldVars[k]; ok {
			os.Setenv(k, old)
		} else {
			os.Unsetenv(k)
		}
	}
}

func loadEnv(envPath string) (map[string]string, error) {
	env := map[string]string{}

	contents, err := os.ReadFile(envPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read env yaml %q: %w", envPath, err)
	}
	if err := yaml.Unmarshal(contents, &env); err != nil {
		return nil, fmt.Errorf("failed to unmarshal env yaml %q: %w", envPath, err)
	}
	return env, nil
}

func New(ctx context.Context, dataDir string, writeToDisk bool) (ret *Datastore, retErr error) {
	emulatorCmd, cmdCancel, err := startEmulator(ctx, dataDir, writeToDisk)
	if err != nil {
		return nil, err
	}
	ret = &Datastore{
		cmd:       emulatorCmd,
		ctxCancel: cmdCancel,
		newEnv:    map[string]string{},
		oldEnv:    map[string]string{},
	}
	defer func() {
		if retErr != nil {
			ret.Close()
		}
	}()

	newEnv, err := loadEnv(filepath.Join(dataDir, "env.yaml"))
	if err != nil {
		return nil, err
	}
	oldEnv := pushEnv(newEnv)
	ret.newEnv = newEnv
	ret.oldEnv = oldEnv

	return
}
