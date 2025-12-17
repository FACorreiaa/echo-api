workspace(name = "cel_go")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "b2038e2de2cace18f032249cb4bb0048abf583a36369fa98f687af1b3f880b26",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.48.1/rules_go-v0.48.1.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.48.1/rules_go-v0.48.1.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "ecba0f04f96b4960a5b250c8e8eeec42281035970aa8852dda73098274d14a1d",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.29.0/bazel-gazelle-v0.29.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.29.0/bazel-gazelle-v0.29.0.tar.gz",
    ],
)

http_archive(
    name = "rules_proto",
    sha256 = "6fb6767d1bef535310547e03247f7518b03487740c11b6c6adb7952033fe1295",
    strip_prefix = "rules_proto-6.0.2",
    url = "https://github.com/bazelbuild/rules_proto/releases/download/6.0.2/rules_proto-6.0.2.tar.gz",
)

# googleapis as of 08/31/2023
http_archive(
    name = "com_google_googleapis",
    sha256 = "5c56500adf7b1b7a3a2ee5ca5b77500617ad80afb808e3d3979f582e64c0523d",
    strip_prefix = "googleapis-25f99371444ea7fd0dc1523ca6925e91cc48a664",
    urls = [
        "https://github.com/googleapis/googleapis/archive/25f99371444ea7fd0dc1523ca6925e91cc48a664.tar.gz",
    ],
)

# protobuf
http_archive(
    name = "com_google_protobuf",
    sha256 = "8242327e5df8c80ba49e4165250b8f79a76bd11765facefaaecfca7747dc8da2",
    strip_prefix = "protobuf-3.21.5",
    urls = ["https://github.com/protocolbuffers/protobuf/archive/v3.21.5.zip"],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")
load("@com_google_googleapis//:repository_rules.bzl", "switched_rules_by_language")
load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

switched_rules_by_language(
    name = "com_google_googleapis_imports",
    go = True,
)

# Do *not* call *_dependencies(), etc, yet.  See comment at the end.

go_repository(
    name = "org_golang_google_protobuf",
    build_file_proto_mode = "disable_global",
    importpath = "google.golang.org/protobuf",
    tag = "v1.34.2",
)

# Generated Google APIs protos for Golang 08/24/2024
go_repository(
    name = "org_golang_google_genproto_googleapis_api",
    build_file_proto_mode = "disable_global",
    importpath = "google.golang.org/genproto/googleapis/api",
    sum = "h1:YcyjlL1PRr2Q17/I0dPk2JmYS5CDXfcdb2Z3YRioEbw=",
    version = "v0.0.0-20240826202546-f6391c0de4c7",
)

# Generated Google APIs protos for Golang 08/24/2024
go_repository(
    name = "org_golang_google_genproto_googleapis_rpc",
    build_file_proto_mode = "disable_global",
    importpath = "google.golang.org/genproto/googleapis/rpc",
    sum = "h1:Kqjm4WpoWvwhMPcrAczoTyMySQmYa9Wy2iL6Con4zn8=",
    version = "v0.0.0-20240823204242-4ba0660f739c",
)

go_repository(
    name = "org_golang_x_text",
    importpath = "golang.org/x/text",
    sum = "h1:a94ExnEXNtEwYLGJSIUxnWoxoRz/ZcCsV63ROupILh4=",
    version = "v0.16.0",
)

# ANTLR v4.13.0
go_repository(
    name = "com_github_antlr4_go_antlr_v4",
    importpath = "github.com/antlr4-go/antlr/v4",
    sum = "h1:lxCg3LAv+EUK6t1i0y1V6/SLeUi0eKEKdhQAlS8TVTI=",
    version = "v4.13.0",
)

# CEL Spec deps
go_repository(
    name = "dev_cel_expr",
    importpath = "cel.dev/expr",
    sum = "h1:56OvJKSH3hDGL0ml5uSxZmz3/3Pq4tJ+fb1unVLAFcY=",
    version = "v0.24.0",
)

# local_repository(
#     name = "dev_cel_expr",
#     path = "</abs/path>/github.com/google/cel-spec",
# )

# strcase deps
go_repository(
    name = "com_github_stoewer_go_strcase",
    importpath = "github.com/stoewer/go-strcase",
    sum = "h1:Z2iHWqGXH00XYgqDmNgQbIBxf3wrNq0F3feEy0ainaU=",
    version = "v1.2.0",
)

# Readline for repl
go_repository(
    name = "com_github_chzyer_readline",
    commit = "62c6fe6193755f722b8b8788aa7357be55a50ff1",  # v1.4
    importpath = "github.com/chzyer/readline",
)

# gopkg.in/yaml.v3 for policy module
go_repository(
    name = "in_gopkg_yaml_v3",
    build_file_proto_mode = "disable_global",
    importpath = "gopkg.in/yaml.v3",
    sum = "h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=",
    version = "v3.0.1",
)

# golang.org/x/exp deps
go_repository(
    name = "org_golang_x_exp",
    importpath = "golang.org/x/exp",
    sum = "h1:mCRnTeVUjcrhlRmO0VK8a6k6Rrf6TF9htwo2pJVSjIU=",
    version = "v0.0.0-20230515195305-f3d0a9c9a5cc",
)

go_repository(
    name = "com_github_google_go_cmp",
    importpath = "github.com/google/go-cmp",
    sum = "h1:O2Tfq5qg4qc4AmwVlvv0oLiVAGB7enBSJ2x2DqQFi38=",
    version = "v0.5.9",
)

# Run the dependencies at the end.  These will silently try to import some
# of the above repositories but at different versions, so ours must come first.
go_rules_dependencies()

go_register_toolchains(version = "1.22.0")

gazelle_dependencies()

load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies")

rules_proto_dependencies()

load("@rules_proto//proto:setup.bzl", "rules_proto_setup")

rules_proto_setup()

load("@rules_proto//proto:toolchains.bzl", "rules_proto_toolchains")

rules_proto_toolchains()

protobuf_deps()
