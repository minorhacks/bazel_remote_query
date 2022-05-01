# bazel_remote_query

## Goal

`bazel query` can be a relatively expensive operation in CI contexts when lots
of files are downloaded via bazel's `WORKSPACE` mechanism. The goal of this
project is to create a pool of workers, a queue of queries for the workers to
service, wrapped with a gRPC service. Using a pool of workers that maintain
cached workspaces should provide faster queries than e.g. ephemeral VMs.