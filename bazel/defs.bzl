"""Helper macros"""

load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

def go_grpc_proto_library(**kwargs):
    go_proto_library(
        compiler = "@io_bazel_rules_go//proto:go_grpc",
        **kwargs
    )
