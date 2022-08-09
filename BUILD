load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:prefix github.com/konveyor/forklift-controller
gazelle(name = "gazelle")

# gazelle:proto disable_global

genrule(
    name = "build-controller",
    srcs = [
        "//cmd/manager",
    ],
    outs = ["manager"],
    cmd = "echo '#!/bin/sh\n\ncp -f $(SRCS) $$1' > \"$@\"",
    executable = 1,
)

