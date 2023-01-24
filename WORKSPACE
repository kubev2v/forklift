load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "ae013bf35bd23234d1dea46b079f1e05ba74ac0321423830119d3e787ec73483",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.36.0/rules_go-v0.36.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.36.0/rules_go-v0.36.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "448e37e0dbf61d6fa8f00aaa12d191745e14f07c31cabfa731f0c8e8a4f41b97",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.28.0/bazel-gazelle-v0.28.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.28.0/bazel-gazelle-v0.28.0.tar.gz",
    ],
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "b1e80761a8a8243d03ebca8845e9cc1ba6c82ce7c5179ce2b295cd36f7e394bf",
    urls = [
        "https://github.com/bazelbuild/rules_docker/releases/download/v0.25.0/rules_docker-v0.25.0.tar.gz",
    ],
)

http_file(
    name = "cirros",
    downloaded_file_path = "cirros.raw",
    sha256 = "cc704ab14342c1c8a8d91b66a7fc611d921c8b8f1aaf4695f9d6463d913fa8d1",
    urls = [
        "https://github.com/cirros-dev/cirros/releases/download/0.6.1/cirros-0.6.1-x86_64-disk.img",
        "https://download.cirros-cloud.net/0.6.1/cirros-0.6.1-x86_64-disk.img",
    ],
)

load(
    "@io_bazel_rules_docker//toolchains/docker:toolchain.bzl",
    docker_toolchain_configure = "toolchain_configure",
)

docker_toolchain_configure(
    name = "docker_config",
    docker_flags = [
        "--log-level=info",
    ],
    docker_path = "${CONTAINER_CMD:-$(command -v podman||command -v docker)}",
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

go_repository(
    name = "co_honnef_go_tools",
    importpath = "honnef.co/go/tools",
    sum = "h1:UoveltGrhghAA7ePc+e+QYDHXrBps2PqFZiHkGR/xK8=",
    version = "v0.0.1-2020.1.4",
)

go_repository(
    name = "com_github_14rcole_gopopulate",
    importpath = "github.com/14rcole/gopopulate",
    sum = "h1:SCbEWT58NSt7d2mcFdvxC9uyrdcTfvBbPLThhkDmXzg=",
    version = "v0.0.0-20180821133914-b175b219e774",
)

go_repository(
    name = "com_github_acarl005_stripansi",
    importpath = "github.com/acarl005/stripansi",
    sum = "h1:licZJFw2RwpHMqeKTCYkitsPqHNxTmd4SNR5r94FGM8=",
    version = "v0.0.0-20180116102854-5a71ef0e047d",
)

go_repository(
    name = "com_github_afex_hystrix_go",
    importpath = "github.com/afex/hystrix-go",
    sum = "h1:rFw4nCn9iMW+Vajsk51NtYIcwSTkXr+JGrMd36kTDJw=",
    version = "v0.0.0-20180502004556-fa1af6a1f4f5",
)

go_repository(
    name = "com_github_agnivade_levenshtein",
    importpath = "github.com/agnivade/levenshtein",
    sum = "h1:3oJU7J3FGFmyhn8KHjmVaZCN5hxTr7GxgRue+sxIXdQ=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_alecthomas_template",
    importpath = "github.com/alecthomas/template",
    sum = "h1:JYp7IbQjafoB+tBA3gMyHYHrpOtNuDiK/uB5uXxq5wM=",
    version = "v0.0.0-20190718012654-fb15b899a751",
)

go_repository(
    name = "com_github_alecthomas_units",
    importpath = "github.com/alecthomas/units",
    sum = "h1:UQZhZ2O0vMHr2cI+DC1Mbh0TJxzA3RcLoMsFw+aXw7E=",
    version = "v0.0.0-20190924025748-f65c72e2690d",
)

go_repository(
    name = "com_github_andreyvit_diff",
    importpath = "github.com/andreyvit/diff",
    sum = "h1:bvNMNQO63//z+xNgfBlViaCIJKLlCJ6/fmUseuG0wVQ=",
    version = "v0.0.0-20170406064948-c7f18ee00883",
)

go_repository(
    name = "com_github_apache_thrift",
    importpath = "github.com/apache/thrift",
    sum = "h1:5hryIiq9gtn+MiLVn0wP37kb/uTeRZgN08WoCsAhIhI=",
    version = "v0.13.0",
)

go_repository(
    name = "com_github_appscode_jsonpatch",
    importpath = "github.com/appscode/jsonpatch",
    sum = "h1:e82Bj+rsBSnpsmjiIGlc9NiKSBpJONZkamk/F8GrCR0=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_armon_circbuf",
    importpath = "github.com/armon/circbuf",
    sum = "h1:QEF07wC0T1rKkctt1RINW/+RMTVmiwxETico2l3gxJA=",
    version = "v0.0.0-20150827004946-bbbad097214e",
)

go_repository(
    name = "com_github_armon_consul_api",
    importpath = "github.com/armon/consul-api",
    sum = "h1:G1bPvciwNyF7IUmKXNt9Ak3m6u9DE1rF+RmtIkBpVdA=",
    version = "v0.0.0-20180202201655-eb2c6b5be1b6",
)

go_repository(
    name = "com_github_armon_go_metrics",
    importpath = "github.com/armon/go-metrics",
    sum = "h1:8GUt8eRujhVEGZFFEjBj46YV4rDjvGrNxb0KMWYkL2I=",
    version = "v0.0.0-20180917152333-f0300d1749da",
)

go_repository(
    name = "com_github_armon_go_radix",
    importpath = "github.com/armon/go-radix",
    sum = "h1:BUAU3CGlLvorLI26FmByPp2eC2qla6E1Tw+scpcg/to=",
    version = "v0.0.0-20180808171621-7fddfc383310",
)

go_repository(
    name = "com_github_aryann_difflib",
    importpath = "github.com/aryann/difflib",
    sum = "h1:pv34s756C4pEXnjgPfGYgdhg/ZdajGhyOvzx8k+23nw=",
    version = "v0.0.0-20170710044230-e206f873d14a",
)

go_repository(
    name = "com_github_asaskevich_govalidator",
    importpath = "github.com/asaskevich/govalidator",
    sum = "h1:idn718Q4B6AGu/h5Sxe66HYVdqdGu2l9Iebqhi/AEoA=",
    version = "v0.0.0-20190424111038-f61b66f89f4a",
)

go_repository(
    name = "com_github_aws_aws_lambda_go",
    importpath = "github.com/aws/aws-lambda-go",
    sum = "h1:SuCy7H3NLyp+1Mrfp+m80jcbi9KYWAs9/BXwppwRDzY=",
    version = "v1.13.3",
)

go_repository(
    name = "com_github_aws_aws_sdk_go",
    importpath = "github.com/aws/aws-sdk-go",
    sum = "h1:0xphMHGMLBrPMfxR2AmVjZKcMEESEgWF8Kru94BNByk=",
    version = "v1.27.0",
)

go_repository(
    name = "com_github_aws_aws_sdk_go_v2",
    importpath = "github.com/aws/aws-sdk-go-v2",
    sum = "h1:qZ+woO4SamnH/eEbjM2IDLhRNwIwND/RQyVlBLp3Jqg=",
    version = "v0.18.0",
)

go_repository(
    name = "com_github_azure_go_ansiterm",
    importpath = "github.com/Azure/go-ansiterm",
    sum = "h1:w+iIsaOQNcT7OZ575w+acHgRric5iCyQh+xv+KJ4HB8=",
    version = "v0.0.0-20170929234023-d6e3b3328b78",
)

go_repository(
    name = "com_github_azure_go_autorest_autorest",
    importpath = "github.com/Azure/go-autorest/autorest",
    sum = "h1:5YWtOnckcudzIw8lPPBcWOnmIFWMtHci1ZWAZulMSx0=",
    version = "v0.9.6",
)

go_repository(
    name = "com_github_azure_go_autorest_autorest_adal",
    importpath = "github.com/Azure/go-autorest/autorest/adal",
    sum = "h1:O1X4oexUxnZCaEUGsvMnr8ZGj8HI37tNezwY4npRqA0=",
    version = "v0.8.2",
)

go_repository(
    name = "com_github_azure_go_autorest_autorest_date",
    importpath = "github.com/Azure/go-autorest/autorest/date",
    sum = "h1:yW+Zlqf26583pE43KhfnhFcdmSWlm5Ew6bxipnr/tbM=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_azure_go_autorest_autorest_mocks",
    importpath = "github.com/Azure/go-autorest/autorest/mocks",
    sum = "h1:qJumjCaCudz+OcqE9/XtEPfvtOjOmKaui4EOpFI6zZc=",
    version = "v0.3.0",
)

go_repository(
    name = "com_github_azure_go_autorest_logger",
    importpath = "github.com/Azure/go-autorest/logger",
    sum = "h1:ruG4BSDXONFRrZZJ2GUXDiUyVpayPmb1GnWeHDdaNKY=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_azure_go_autorest_tracing",
    importpath = "github.com/Azure/go-autorest/tracing",
    sum = "h1:TRn4WjSnkcSy5AEG3pnbtFSwNtwzjr4VYyQflFE619k=",
    version = "v0.5.0",
)

go_repository(
    name = "com_github_beorn7_perks",
    importpath = "github.com/beorn7/perks",
    sum = "h1:VlbKKnNfV8bJzeqoa4cOKqO6bYr3WgKZxO8Z16+hsOM=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_bgentry_speakeasy",
    importpath = "github.com/bgentry/speakeasy",
    sum = "h1:ByYyxL9InA1OWqxJqqp2A5pYHUrCiAL6K3J+LKSsQkY=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_bketelsen_crypt",
    importpath = "github.com/bketelsen/crypt",
    sum = "h1:+0HFd5KSZ/mm3JmhmrDukiId5iR6w4+BdFtfSy4yWIc=",
    version = "v0.0.3-0.20200106085610-5cbc8cc4026c",
)

go_repository(
    name = "com_github_blang_semver",
    importpath = "github.com/blang/semver",
    sum = "h1:cQNTCjp13qL8KC3Nbxr/y2Bqb63oX6wdnnjpJbkM4JQ=",
    version = "v3.5.1+incompatible",
)

go_repository(
    name = "com_github_boltdb_bolt",
    importpath = "github.com/boltdb/bolt",
    sum = "h1:JQmyP4ZBrce+ZQu0dY660FMfatumYDLun9hBCUVIkF4=",
    version = "v1.3.1",
)

go_repository(
    name = "com_github_brancz_gojsontoyaml",
    importpath = "github.com/brancz/gojsontoyaml",
    sum = "h1:DMb8SuAL9+demT8equqMMzD8C/uxqWmj4cgV7ufrpQo=",
    version = "v0.0.0-20190425155809-e8bd32d46b3d",
)

go_repository(
    name = "com_github_burntsushi_toml",
    importpath = "github.com/BurntSushi/toml",
    sum = "h1:WXkYYl6Yr3qBf1K79EBnL4mak0OimBfB0XUf9Vl28OQ=",
    version = "v0.3.1",
)

go_repository(
    name = "com_github_burntsushi_xgb",
    importpath = "github.com/BurntSushi/xgb",
    sum = "h1:1BDTz0u9nC3//pOCMdNH+CiXJVYJh5UQNCOBG7jbELc=",
    version = "v0.0.0-20160522181843-27f122750802",
)

go_repository(
    name = "com_github_campoy_embedmd",
    importpath = "github.com/campoy/embedmd",
    sum = "h1:V4kI2qTJJLf4J29RzI/MAt2c3Bl4dQSYPuflzwFH2hY=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_casbin_casbin_v2",
    importpath = "github.com/casbin/casbin/v2",
    sum = "h1:bTwon/ECRx9dwBy2ewRVr5OiqjeXSGiTUY74sDPQi/g=",
    version = "v2.1.2",
)

go_repository(
    name = "com_github_cenkalti_backoff",
    importpath = "github.com/cenkalti/backoff",
    sum = "h1:tNowT99t7UNflLxfYYSlKYsBpXdEet03Pg2g16Swow4=",
    version = "v2.2.1+incompatible",
)

go_repository(
    name = "com_github_census_instrumentation_opencensus_proto",
    importpath = "github.com/census-instrumentation/opencensus-proto",
    sum = "h1:glEXhBS5PSLLv4IXzLA5yPRVX4bilULVyxxbrfOtDAk=",
    version = "v0.2.1",
)

go_repository(
    name = "com_github_certifi_gocertifi",
    importpath = "github.com/certifi/gocertifi",
    sum = "h1:MmeatFT1pTPSVb4nkPmBFN/LRZ97vPjsFKsZrU3KKTs=",
    version = "v0.0.0-20180905225744-ee1a9a0726d2",
)

go_repository(
    name = "com_github_cespare_xxhash",
    importpath = "github.com/cespare/xxhash",
    sum = "h1:a6HrQnmkObjyL+Gs60czilIUGqrzKutQD6XZog3p+ko=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_cespare_xxhash_v2",
    importpath = "github.com/cespare/xxhash/v2",
    sum = "h1:6MnRN8NT7+YBpUIWxHtefFZOKTAPgGjpQSxqLNn0+qY=",
    version = "v2.1.1",
)

go_repository(
    name = "com_github_chzyer_logex",
    importpath = "github.com/chzyer/logex",
    sum = "h1:Swpa1K6QvQznwJRcfTfQJmTE72DqScAa40E+fbHEXEE=",
    version = "v1.1.10",
)

go_repository(
    name = "com_github_chzyer_readline",
    importpath = "github.com/chzyer/readline",
    sum = "h1:fY5BOSpyZCqRo5OhCuC+XN+r/bBCmeuuJtjz+bCNIf8=",
    version = "v0.0.0-20180603132655-2972be24d48e",
)

go_repository(
    name = "com_github_chzyer_test",
    importpath = "github.com/chzyer/test",
    sum = "h1:q763qf9huN11kDQavWsoZXJNW3xEE4JJyHa5Q25/sd8=",
    version = "v0.0.0-20180213035817-a1ea475d72b1",
)

go_repository(
    name = "com_github_clbanning_x2j",
    importpath = "github.com/clbanning/x2j",
    sum = "h1:EdRZT3IeKQmfCSrgo8SZ8V3MEnskuJP0wCYNpe+aiXo=",
    version = "v0.0.0-20191024224557-825249438eec",
)

go_repository(
    name = "com_github_client9_misspell",
    importpath = "github.com/client9/misspell",
    sum = "h1:ta993UF76GwbvJcIo3Y68y/M3WxlpEHPWIGDkJYwzJI=",
    version = "v0.3.4",
)

go_repository(
    name = "com_github_cncf_udpa_go",
    importpath = "github.com/cncf/udpa/go",
    sum = "h1:WBZRG4aNOuI15bLRrCgN8fCq8E5Xuty6jGbmSNEvSsU=",
    version = "v0.0.0-20191209042840-269d4d468f6f",
)

go_repository(
    name = "com_github_cockroachdb_datadriven",
    importpath = "github.com/cockroachdb/datadriven",
    sum = "h1:OaNxuTZr7kxeODyLWsRMC+OD03aFUH+mW6r2d+MWa5Y=",
    version = "v0.0.0-20190809214429-80d97fb3cbaa",
)

go_repository(
    name = "com_github_codahale_hdrhistogram",
    importpath = "github.com/codahale/hdrhistogram",
    sum = "h1:qMd81Ts1T2OTKmB4acZcyKaMtRnY5Y44NuXGX2GFJ1w=",
    version = "v0.0.0-20161010025455-3a0bb77429bd",
)

go_repository(
    name = "com_github_container_storage_interface_spec",
    importpath = "github.com/container-storage-interface/spec",
    sum = "h1:bD9KIVgaVKKkQ/UbVUY9kCaH/CJbhNxe0eeB4JeJV2s=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_containerd_cgroups",
    importpath = "github.com/containerd/cgroups",
    sum = "h1:tSNMc+rJDfmYntojat8lljbt1mgKNpTxUZJsSzJ9Y1s=",
    version = "v0.0.0-20190919134610-bf292b21730f",
)

go_repository(
    name = "com_github_containerd_console",
    importpath = "github.com/containerd/console",
    sum = "h1:uict5mhHFTzKLUCufdSLym7z/J0CbBJT59lYbP9wtbg=",
    version = "v0.0.0-20180822173158-c12b1e7919c1",
)

go_repository(
    name = "com_github_containerd_containerd",
    importpath = "github.com/containerd/containerd",
    sum = "h1:ForxmXkA6tPIvffbrDAcPUIB32QgXkt2XFj+F0UxetA=",
    version = "v1.3.2",
)

go_repository(
    name = "com_github_containerd_continuity",
    importpath = "github.com/containerd/continuity",
    sum = "h1:NmTXa/uVnDyp0TY5MKi197+3HWcnYWfnHGyaFthlnGw=",
    version = "v0.0.0-20190827140505-75bee3e2ccb6",
)

go_repository(
    name = "com_github_containerd_fifo",
    importpath = "github.com/containerd/fifo",
    sum = "h1:PUD50EuOMkXVcpBIA/R95d56duJR9VxhwncsFbNnxW4=",
    version = "v0.0.0-20190226154929-a9fb20d87448",
)

go_repository(
    name = "com_github_containerd_go_runc",
    importpath = "github.com/containerd/go-runc",
    sum = "h1:esQOJREg8nw8aXj6uCN5dfW5cKUBiEJ/+nni1Q/D/sw=",
    version = "v0.0.0-20180907222934-5a6d9f37cfa3",
)

go_repository(
    name = "com_github_containerd_ttrpc",
    importpath = "github.com/containerd/ttrpc",
    sum = "h1:dlfGmNcE3jDAecLqwKPMNX6nk2qh1c1Vg1/YTzpOOF4=",
    version = "v0.0.0-20190828154514-0e0f228740de",
)

go_repository(
    name = "com_github_containerd_typeurl",
    importpath = "github.com/containerd/typeurl",
    sum = "h1:JNn81o/xG+8NEo3bC/vx9pbi/g2WI8mtP2/nXzu297Y=",
    version = "v0.0.0-20180627222232-a93fcdb778cd",
)

go_repository(
    name = "com_github_containernetworking_cni",
    importpath = "github.com/containernetworking/cni",
    sum = "h1:fE3r16wpSEyaqY4Z4oFrLMmIGfBYIKpPrHK31EJ9FzE=",
    version = "v0.7.1",
)

go_repository(
    name = "com_github_containers_image_v5",
    importpath = "github.com/containers/image/v5",
    sum = "h1:h1FCOXH6Ux9/p/E4rndsQOC4yAdRU0msRTfLVeQ7FDQ=",
    version = "v5.5.1",
)

go_repository(
    name = "com_github_containers_libtrust",
    importpath = "github.com/containers/libtrust",
    sum = "h1:Q8ePgVfHDplZ7U33NwHZkrVELsZP5fYj9pM5WBZB2GE=",
    version = "v0.0.0-20190913040956-14b96171aa3b",
)

go_repository(
    name = "com_github_containers_ocicrypt",
    importpath = "github.com/containers/ocicrypt",
    sum = "h1:Q0/IPs8ohfbXNxEfyJ2pFVmvJu5BhqJUAmc6ES9NKbo=",
    version = "v1.0.2",
)

go_repository(
    name = "com_github_containers_storage",
    importpath = "github.com/containers/storage",
    sum = "h1:tw/uKRPDnmVrluIzer3dawTFG/bTJLP8IEUyHFhltYk=",
    version = "v1.20.2",
)

go_repository(
    name = "com_github_coreos_bbolt",
    importpath = "github.com/coreos/bbolt",
    sum = "h1:wZwiHHUieZCquLkDL0B8UhzreNWsPHooDAG3q34zk0s=",
    version = "v1.3.2",
)

go_repository(
    name = "com_github_coreos_etcd",
    importpath = "github.com/coreos/etcd",
    sum = "h1:8F3hqu9fGYLBifCmRCJsicFqDx/D68Rt3q1JMazcgBQ=",
    version = "v3.3.13+incompatible",
)

go_repository(
    name = "com_github_coreos_go_etcd",
    importpath = "github.com/coreos/go-etcd",
    sum = "h1:bXhRBIXoTm9BYHS3gE0TtQuyNZyeEMux2sDi4oo5YOo=",
    version = "v2.0.0+incompatible",
)

go_repository(
    name = "com_github_coreos_go_oidc",
    importpath = "github.com/coreos/go-oidc",
    sum = "h1:sdJrfw8akMnCuUlaZU3tE/uYXFgfqom8DBE9so9EBsM=",
    version = "v2.1.0+incompatible",
)

go_repository(
    name = "com_github_coreos_go_semver",
    importpath = "github.com/coreos/go-semver",
    sum = "h1:wkHLiw0WNATZnSG7epLsujiMCgPAc9xhjJ4tgnAxmfM=",
    version = "v0.3.0",
)

go_repository(
    name = "com_github_coreos_go_systemd",
    importpath = "github.com/coreos/go-systemd",
    sum = "h1:W8b4lQ4tFF21aspRGoBuCNV6V2fFJBF+pm1J6OY8Lys=",
    version = "v0.0.0-20190620071333-e64a0ec8b42a",
)

go_repository(
    name = "com_github_coreos_pkg",
    importpath = "github.com/coreos/pkg",
    sum = "h1:lBNOc5arjvs8E5mO2tbpBpLoyyu8B6e44T7hJy6potg=",
    version = "v0.0.0-20180928190104-399ea9e2e55f",
)

go_repository(
    name = "com_github_coreos_prometheus_operator",
    importpath = "github.com/coreos/prometheus-operator",
    sum = "h1:kd7mysk8mCdwquBcPLyuRoRFNJCpgezXu8yUvIYE2nc=",
    version = "v0.35.0",
)

go_repository(
    name = "com_github_cpuguy83_go_md2man",
    importpath = "github.com/cpuguy83/go-md2man",
    sum = "h1:BSKMNlYxDvnunlTymqtgONjNnaRV1sTpcovwwjF22jk=",
    version = "v1.0.10",
)

go_repository(
    name = "com_github_cpuguy83_go_md2man_v2",
    importpath = "github.com/cpuguy83/go-md2man/v2",
    sum = "h1:EoUDS0afbrsXAZ9YQ9jdu/mZ2sXgT1/2yyNng4PGlyM=",
    version = "v2.0.0",
)

go_repository(
    name = "com_github_creack_pty",
    importpath = "github.com/creack/pty",
    sum = "h1:6pwm8kMQKCmgUg0ZHTm5+/YvRK0s3THD/28+T6/kk4A=",
    version = "v1.1.7",
)

go_repository(
    name = "com_github_davecgh_go_spew",
    importpath = "github.com/davecgh/go-spew",
    sum = "h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_davecgh_go_xdr",
    importpath = "github.com/davecgh/go-xdr",
    sum = "h1:qg9VbHo1TlL0KDM0vYvBG9EY0X0Yku5WYIPoFWt8f6o=",
    version = "v0.0.0-20161123171359-e6a2ba005892",
)

go_repository(
    name = "com_github_dgrijalva_jwt_go",
    importpath = "github.com/dgrijalva/jwt-go",
    sum = "h1:7qlOGliEKZXTDg6OTjfoBKDXWrumCAMpl/TFQ4/5kLM=",
    version = "v3.2.0+incompatible",
)

go_repository(
    name = "com_github_dgryski_go_sip13",
    importpath = "github.com/dgryski/go-sip13",
    sum = "h1:RMLoZVzv4GliuWafOuPuQDKSm1SJph7uCRnnS61JAn4=",
    version = "v0.0.0-20181026042036-e10d5fee7954",
)

go_repository(
    name = "com_github_docker_distribution",
    importpath = "github.com/docker/distribution",
    sum = "h1:a5mlkVzth6W5A4fOsS3D2EO5BUmsJpcB+cRlLU7cSug=",
    version = "v2.7.1+incompatible",
)

go_repository(
    name = "com_github_docker_docker",
    importpath = "github.com/docker/docker",
    sum = "h1:Sm8iD2lifO31DwXfkGzq8VgA7rwxPjRsYmeo0K/dF9Y=",
    version = "v1.4.2-0.20191219165747-a9416c67da9f",
)

go_repository(
    name = "com_github_docker_docker_credential_helpers",
    importpath = "github.com/docker/docker-credential-helpers",
    sum = "h1:zI2p9+1NQYdnG6sMU26EX4aVGlqbInSQxQXLvzJ4RPQ=",
    version = "v0.6.3",
)

go_repository(
    name = "com_github_docker_go_connections",
    importpath = "github.com/docker/go-connections",
    sum = "h1:El9xVISelRB7BuFusrZozjnkIM5YnzCViNKohAFqRJQ=",
    version = "v0.4.0",
)

go_repository(
    name = "com_github_docker_go_metrics",
    importpath = "github.com/docker/go-metrics",
    sum = "h1:AgB/0SvBxihN0X8OR4SjsblXkbMvalQ8cjmtKQ2rQV8=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_docker_go_units",
    importpath = "github.com/docker/go-units",
    sum = "h1:3uh0PgVws3nIA0Q+MwDC8yjEPf9zjRfZZWXZYDct3Tw=",
    version = "v0.4.0",
)

go_repository(
    name = "com_github_docker_libnetwork",
    importpath = "github.com/docker/libnetwork",
    sum = "h1:rACxqwRsHD075Vb7qTimAgMSKWoM5zER0Dhws/SgCVo=",
    version = "v0.0.0-20190731215715-7f13a5c99f4b",
)

go_repository(
    name = "com_github_docker_libtrust",
    importpath = "github.com/docker/libtrust",
    sum = "h1:UhxFibDNY/bfvqU5CAUmr9zpesgbU6SWc8/B4mflAE4=",
    version = "v0.0.0-20160708172513-aabc10ec26b7",
)

go_repository(
    name = "com_github_docker_spdystream",
    importpath = "github.com/docker/spdystream",
    sum = "h1:ZfSZ3P3BedhKGUhzj7BQlPSU4OvT6tfOKe3DVHzOA7s=",
    version = "v0.0.0-20181023171402-6480d4af844c",
)

go_repository(
    name = "com_github_docopt_docopt_go",
    importpath = "github.com/docopt/docopt-go",
    sum = "h1:bWDMxwH3px2JBh6AyO7hdCn/PkvCZXii8TGj7sbtEbQ=",
    version = "v0.0.0-20180111231733-ee0de3bc6815",
)

go_repository(
    name = "com_github_dustin_go_humanize",
    importpath = "github.com/dustin/go-humanize",
    sum = "h1:VSnTsYCnlFHaM2/igO1h6X3HA71jcobQuxemgkq4zYo=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_eapache_go_resiliency",
    importpath = "github.com/eapache/go-resiliency",
    sum = "h1:1NtRmCAqadE2FN4ZcN6g90TP3uk8cg9rn9eNK2197aU=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_eapache_go_xerial_snappy",
    importpath = "github.com/eapache/go-xerial-snappy",
    sum = "h1:YEetp8/yCZMuEPMUDHG0CW/brkkEp8mzqk2+ODEitlw=",
    version = "v0.0.0-20180814174437-776d5712da21",
)

go_repository(
    name = "com_github_eapache_queue",
    importpath = "github.com/eapache/queue",
    sum = "h1:YOEu7KNc61ntiQlcEeUIoDTJ2o8mQznoNvUhiigpIqc=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_edsrzf_mmap_go",
    importpath = "github.com/edsrzf/mmap-go",
    sum = "h1:CEBF7HpRnUCSJgGUb5h1Gm7e3VkmVDrR8lvWVLtrOFw=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_elazarl_go_bindata_assetfs",
    importpath = "github.com/elazarl/go-bindata-assetfs",
    sum = "h1:G/bYguwHIzWq9ZoyUQqrjTmJbbYn3j3CKKpKinvZLFk=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_elazarl_goproxy",
    importpath = "github.com/elazarl/goproxy",
    sum = "h1:yY9rWGoXv1U5pl4gxqlULARMQD7x0QG85lqEXTWysik=",
    version = "v0.0.0-20190911111923-ecfe977594f1",
)

go_repository(
    name = "com_github_elazarl_goproxy_ext",
    importpath = "github.com/elazarl/goproxy/ext",
    sum = "h1:dWB6v3RcOy03t/bUadywsbyrQwCqZeNIEX6M1OtSZOM=",
    version = "v0.0.0-20190711103511-473e67f1d7d2",
)

go_repository(
    name = "com_github_emicklei_go_restful",
    importpath = "github.com/emicklei/go-restful",
    sum = "h1:l6Soi8WCOOVAeCo4W98iBFC6Og7/X8bpRt51oNLZ2C8=",
    version = "v2.10.0+incompatible",
)

go_repository(
    name = "com_github_emicklei_go_restful_openapi",
    importpath = "github.com/emicklei/go-restful-openapi",
    sum = "h1:ohRZ1yEZERGzqaozBgxa3A0lt6c6KF14xhs3IL9ECwg=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_emicklei_go_restful_swagger12",
    importpath = "github.com/emicklei/go-restful-swagger12",
    sum = "h1:V94anc0ZG3Pa/cAMwP2m1aQW3+/FF8Qmw/GsFyTJAp4=",
    version = "v0.0.0-20170926063155-7524189396c6",
)

go_repository(
    name = "com_github_envoyproxy_go_control_plane",
    importpath = "github.com/envoyproxy/go-control-plane",
    sum = "h1:rEvIZUSZ3fx39WIi3JkQqQBitGwpELBIYWeBVh6wn+E=",
    version = "v0.9.4",
)

go_repository(
    name = "com_github_envoyproxy_protoc_gen_validate",
    importpath = "github.com/envoyproxy/protoc-gen-validate",
    sum = "h1:EQciDnbrYxy13PgWoY8AqoxGiPrpgBZ1R8UNe3ddc+A=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_evanphx_json_patch",
    importpath = "github.com/evanphx/json-patch",
    sum = "h1:kLcOMZeuLAJvL2BPWLMIj5oaZQobrkAqrL+WFZwQses=",
    version = "v4.9.0+incompatible",
)

go_repository(
    name = "com_github_fatih_color",
    importpath = "github.com/fatih/color",
    sum = "h1:8xPHl4/q1VyqGIPif1F+1V3Y3lSmrq01EabUW3CoW5s=",
    version = "v1.9.0",
)

go_repository(
    name = "com_github_fortytw2_leaktest",
    importpath = "github.com/fortytw2/leaktest",
    sum = "h1:u8491cBMTQ8ft8aeV+adlcytMZylmA5nnwwkRZjI8vw=",
    version = "v1.3.0",
)

go_repository(
    name = "com_github_franela_goblin",
    importpath = "github.com/franela/goblin",
    sum = "h1:gb2Z18BhTPJPpLQWj4T+rfKHYCHxRHCtRxhKKjRidVw=",
    version = "v0.0.0-20200105215937-c9ffbefa60db",
)

go_repository(
    name = "com_github_franela_goreq",
    importpath = "github.com/franela/goreq",
    sum = "h1:a9ENSRDFBUPkJ5lCgVZh26+ZbGyoVJG7yb5SSzF5H54=",
    version = "v0.0.0-20171204163338-bcd34c9993f8",
)

go_repository(
    name = "com_github_fsnotify_fsnotify",
    importpath = "github.com/fsnotify/fsnotify",
    sum = "h1:hsms1Qyu0jgnwNXIxa+/V/PDsU6CfLf6CNO8H7IWoS4=",
    version = "v1.4.9",
)

go_repository(
    name = "com_github_fsouza_go_dockerclient",
    importpath = "github.com/fsouza/go-dockerclient",
    sum = "h1:B94x7idrPK5yFSMWliAvakUQlTDLgO7iJyHtpbYxGGM=",
    version = "v0.0.0-20171004212419-da3951ba2e9e",
)

go_repository(
    name = "com_github_fullsailor_pkcs7",
    importpath = "github.com/fullsailor/pkcs7",
    sum = "h1:RDBNVkRviHZtvDvId8XSGPu3rmpmSe+wKRcEWNgsfWU=",
    version = "v0.0.0-20190404230743-d7302db945fa",
)

go_repository(
    name = "com_github_getsentry_raven_go",
    importpath = "github.com/getsentry/raven-go",
    sum = "h1:F2m41rgyxoveZKD+Z6xwyAbtdNeVvhpi9BpQLvt5oRU=",
    version = "v0.0.0-20190513200303-c977f96e1095",
)

go_repository(
    name = "com_github_ghodss_yaml",
    importpath = "github.com/ghodss/yaml",
    sum = "h1:wQHKEahhL6wmXdzwWG11gIVCkOv05bNOh+Rxn0yngAk=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_gin_contrib_cors",
    importpath = "github.com/gin-contrib/cors",
    sum = "h1:doAsuITavI4IOcd0Y19U4B+O0dNWihRyX//nn4sEmgA=",
    version = "v1.3.1",
)

go_repository(
    name = "com_github_gin_contrib_sse",
    importpath = "github.com/gin-contrib/sse",
    sum = "h1:Y/yl/+YNO8GZSjAhjMsSuLt29uWRFHdHYUb5lYOV9qE=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_gin_gonic_gin",
    importpath = "github.com/gin-gonic/gin",
    sum = "h1:Tg03T9yM2xa8j6I3Z3oqLaQRSmKvxPd6g/2HJ6zICFA=",
    version = "v1.7.2",
)

go_repository(
    name = "com_github_globalsign_mgo",
    importpath = "github.com/globalsign/mgo",
    sum = "h1:DujepqpGd1hyOd7aW59XpK7Qymp8iy83xq74fLr21is=",
    version = "v0.0.0-20181015135952-eeefdecb41b8",
)

go_repository(
    name = "com_github_go_bindata_go_bindata",
    importpath = "github.com/go-bindata/go-bindata",
    sum = "h1:5vjJMVhowQdPzjE1LdxyFF7YFTXg5IgGVW4gBr5IbvE=",
    version = "v3.1.2+incompatible",
)

go_repository(
    name = "com_github_go_gl_glfw",
    importpath = "github.com/go-gl/glfw",
    sum = "h1:QbL/5oDUmRBzO9/Z7Seo6zf912W/a6Sr4Eu0G/3Jho0=",
    version = "v0.0.0-20190409004039-e6da0acd62b1",
)

go_repository(
    name = "com_github_go_gl_glfw_v3_3_glfw",
    importpath = "github.com/go-gl/glfw/v3.3/glfw",
    sum = "h1:WtGNWLvXpe6ZudgnXrq0barxBImvnnJoMEhXAzcbM0I=",
    version = "v0.0.0-20200222043503-6f7a984d4dc4",
)

go_repository(
    name = "com_github_go_kit_kit",
    importpath = "github.com/go-kit/kit",
    sum = "h1:dXFJfIHVvUcpSgDOV+Ne6t7jXri8Tfv2uOLHUZ2XNuo=",
    version = "v0.10.0",
)

go_repository(
    name = "com_github_go_kit_log",
    importpath = "github.com/go-kit/log",
    sum = "h1:DGJh0Sm43HbOeYDNnVZFl8BvcYVvjD5bqYJvp0REbwQ=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_go_logfmt_logfmt",
    importpath = "github.com/go-logfmt/logfmt",
    sum = "h1:TrB8swr/68K7m9CcGut2g3UOihhbcbiMAYiuTXdEih4=",
    version = "v0.5.0",
)

go_repository(
    name = "com_github_go_logr_logr",
    importpath = "github.com/go-logr/logr",
    sum = "h1:q4c+kbcR0d5rSurhBR8dIgieOaYpXtsdTYfx22Cu6rs=",
    version = "v0.3.0",
)

go_repository(
    name = "com_github_go_logr_zapr",
    importpath = "github.com/go-logr/zapr",
    sum = "h1:iyiCRZ29uPmbO7mWIjOEiYMXrTxZWTyK4tCatLyGpUY=",
    version = "v0.3.0",
)

go_repository(
    name = "com_github_go_openapi_analysis",
    importpath = "github.com/go-openapi/analysis",
    sum = "h1:8b2ZgKfKIUTVQpTb77MoRDIMEIwvDVw40o3aOXdfYzI=",
    version = "v0.19.5",
)

go_repository(
    name = "com_github_go_openapi_errors",
    importpath = "github.com/go-openapi/errors",
    sum = "h1:a2kIyV3w+OS3S97zxUndRVD46+FhGOUBDFY7nmu4CsY=",
    version = "v0.19.2",
)

go_repository(
    name = "com_github_go_openapi_jsonpointer",
    importpath = "github.com/go-openapi/jsonpointer",
    sum = "h1:gihV7YNZK1iK6Tgwwsxo2rJbD1GTbdm72325Bq8FI3w=",
    version = "v0.19.3",
)

go_repository(
    name = "com_github_go_openapi_jsonreference",
    importpath = "github.com/go-openapi/jsonreference",
    sum = "h1:5cxNfTy0UVC3X8JL5ymxzyoUZmo8iZb+jeTWn7tUa8o=",
    version = "v0.19.3",
)

go_repository(
    name = "com_github_go_openapi_loads",
    importpath = "github.com/go-openapi/loads",
    sum = "h1:5I4CCSqoWzT+82bBkNIvmLc0UOsoKKQ4Fz+3VxOB7SY=",
    version = "v0.19.4",
)

go_repository(
    name = "com_github_go_openapi_runtime",
    importpath = "github.com/go-openapi/runtime",
    sum = "h1:csnOgcgAiuGoM/Po7PEpKDoNulCcF3FGbSnbHfxgjMI=",
    version = "v0.19.4",
)

go_repository(
    name = "com_github_go_openapi_spec",
    importpath = "github.com/go-openapi/spec",
    sum = "h1:ixzUSnHTd6hCemgtAJgluaTSGYpLNpJY4mA2DIkdOAo=",
    version = "v0.19.4",
)

go_repository(
    name = "com_github_go_openapi_strfmt",
    importpath = "github.com/go-openapi/strfmt",
    sum = "h1:eRfyY5SkaNJCAwmmMcADjY31ow9+N7MCLW7oRkbsINA=",
    version = "v0.19.3",
)

go_repository(
    name = "com_github_go_openapi_swag",
    importpath = "github.com/go-openapi/swag",
    sum = "h1:lTz6Ys4CmqqCQmZPBlbQENR1/GucA2bzYTE12Pw4tFY=",
    version = "v0.19.5",
)

go_repository(
    name = "com_github_go_openapi_validate",
    importpath = "github.com/go-openapi/validate",
    sum = "h1:QhCBKRYqZR+SKo4gl1lPhPahope8/RLt6EVgY8X80w0=",
    version = "v0.19.5",
)

go_repository(
    name = "com_github_go_playground_assert_v2",
    importpath = "github.com/go-playground/assert/v2",
    sum = "h1:MsBgLAaY856+nPRTKrp3/OZK38U/wa0CcBYNjji3q3A=",
    version = "v2.0.1",
)

go_repository(
    name = "com_github_go_playground_locales",
    importpath = "github.com/go-playground/locales",
    sum = "h1:HyWk6mgj5qFqCT5fjGBuRArbVDfE4hi8+e8ceBS/t7Q=",
    version = "v0.13.0",
)

go_repository(
    name = "com_github_go_playground_universal_translator",
    importpath = "github.com/go-playground/universal-translator",
    sum = "h1:icxd5fm+REJzpZx7ZfpaD876Lmtgy7VtROAbHHXk8no=",
    version = "v0.17.0",
)

go_repository(
    name = "com_github_go_playground_validator_v10",
    importpath = "github.com/go-playground/validator/v10",
    sum = "h1:pH2c5ADXtd66mxoE0Zm9SUhxE20r7aM3F26W0hOn+GE=",
    version = "v10.4.1",
)

go_repository(
    name = "com_github_go_sql_driver_mysql",
    importpath = "github.com/go-sql-driver/mysql",
    sum = "h1:7LxgVwFb2hIQtMm87NdgAVfXjnt4OePseqT1tKx+opk=",
    version = "v1.4.0",
)

go_repository(
    name = "com_github_go_stack_stack",
    importpath = "github.com/go-stack/stack",
    sum = "h1:5SgMzNM5HxrEjV0ww2lTmX6E2Izsfxas4+YHWRs3Lsk=",
    version = "v1.8.0",
)

go_repository(
    name = "com_github_gobuffalo_flect",
    importpath = "github.com/gobuffalo/flect",
    sum = "h1:PAVD7sp0KOdfswjAw9BpLCU9hXo7wFSzgpQ+zNeks/A=",
    version = "v0.2.2",
)

go_repository(
    name = "com_github_godbus_dbus",
    importpath = "github.com/godbus/dbus",
    sum = "h1:BWhy2j3IXJhjCbC68FptL43tDKIq8FladmaTs3Xs7Z8=",
    version = "v0.0.0-20190422162347-ade71ed3457e",
)

go_repository(
    name = "com_github_gogo_googleapis",
    importpath = "github.com/gogo/googleapis",
    sum = "h1:kFkMAZBNAn4j7K0GiZr8cRYzejq68VbheufiV3YuyFI=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_gogo_protobuf",
    importpath = "github.com/gogo/protobuf",
    sum = "h1:Ov1cvc58UF3b5XjBnZv7+opcTcQFZebYjWzi34vdm4Q=",
    version = "v1.3.2",
)

go_repository(
    name = "com_github_golang_glog",
    importpath = "github.com/golang/glog",
    sum = "h1:VKtxabqXZkF25pY9ekfRL6a582T4P37/31XEstQ5p58=",
    version = "v0.0.0-20160126235308-23def4e6c14b",
)

go_repository(
    name = "com_github_golang_groupcache",
    importpath = "github.com/golang/groupcache",
    sum = "h1:1r7pUrabqp18hOBcwBwiTsbnFeTZHV9eER/QT5JVZxY=",
    version = "v0.0.0-20200121045136-8c9f03a8e57e",
)

go_repository(
    name = "com_github_golang_lint",
    importpath = "github.com/golang/lint",
    sum = "h1:2hRPrmiwPrp3fQX967rNJIhQPtiGXdlQWAxKbKw3VHA=",
    version = "v0.0.0-20180702182130-06c8688daad7",
)

go_repository(
    name = "com_github_golang_mock",
    importpath = "github.com/golang/mock",
    sum = "h1:l75CXGRSwbaYNpl/Z2X1XIIAMSCquvXgpVZDhwEIJsc=",
    version = "v1.4.4",
)

go_repository(
    name = "com_github_golang_protobuf",
    importpath = "github.com/golang/protobuf",
    sum = "h1:JjCZWpVbqXDqFVmTfYWEVTMIYrL/NPdPSCHPJ0T/raM=",
    version = "v1.4.3",
)

go_repository(
    name = "com_github_golang_snappy",
    importpath = "github.com/golang/snappy",
    sum = "h1:aeE13tS0IiQgFjYdoL8qN3K1N2bXXtI6Vi51/y7BpMw=",
    version = "v0.0.2",
)

go_repository(
    name = "com_github_gonum_blas",
    importpath = "github.com/gonum/blas",
    sum = "h1:Q0Jsdxl5jbxouNs1TQYt0gxesYMU4VXRbsTlgDloZ50=",
    version = "v0.0.0-20181208220705-f22b278b28ac",
)

go_repository(
    name = "com_github_gonum_floats",
    importpath = "github.com/gonum/floats",
    sum = "h1:EvokxLQsaaQjcWVWSV38221VAK7qc2zhaO17bKys/18=",
    version = "v0.0.0-20181209220543-c233463c7e82",
)

go_repository(
    name = "com_github_gonum_graph",
    importpath = "github.com/gonum/graph",
    sum = "h1:NcVXNHJrvrcAv8SVYKzKT2zwtEXU1DK2J+azsK7oz2A=",
    version = "v0.0.0-20170401004347-50b27dea7ebb",
)

go_repository(
    name = "com_github_gonum_internal",
    importpath = "github.com/gonum/internal",
    sum = "h1:8jtTdc+Nfj9AR+0soOeia9UZSvYBvETVHZrugUowJ7M=",
    version = "v0.0.0-20181124074243-f884aa714029",
)

go_repository(
    name = "com_github_gonum_lapack",
    importpath = "github.com/gonum/lapack",
    sum = "h1:7qnwS9+oeSiOIsiUMajT+0R7HR6hw5NegnKPmn/94oI=",
    version = "v0.0.0-20181123203213-e4cdc5a0bff9",
)

go_repository(
    name = "com_github_gonum_matrix",
    importpath = "github.com/gonum/matrix",
    sum = "h1:V2IgdyerlBa/MxaEFRbV5juy/C3MGdj4ePi+g6ePIp4=",
    version = "v0.0.0-20181209220409-c518dec07be9",
)

go_repository(
    name = "com_github_google_btree",
    importpath = "github.com/google/btree",
    sum = "h1:0udJVsspx3VBr5FwtLhQQtuAsVc79tTq0ocGIPAU6qo=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_google_go_cmp",
    importpath = "github.com/google/go-cmp",
    sum = "h1:Khx7svrCpmxxtHBq5j2mp/xVjsi8hQMfNLvJFAlrGgU=",
    version = "v0.5.5",
)

go_repository(
    name = "com_github_google_gofuzz",
    importpath = "github.com/google/gofuzz",
    sum = "h1:Hsa8mG0dQ46ij8Sl2AYJDUv1oA9/d6Vk+3LG99Oe02g=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_google_martian",
    importpath = "github.com/google/martian",
    sum = "h1:/CP5g8u/VJHijgedC/Legn3BAbAaWPgecwXBIDzw5no=",
    version = "v2.1.0+incompatible",
)

go_repository(
    name = "com_github_google_martian_v3",
    importpath = "github.com/google/martian/v3",
    sum = "h1:pMen7vLs8nvgEYhywH3KDWJIJTeEr2ULsVWHWYHQyBs=",
    version = "v3.0.0",
)

go_repository(
    name = "com_github_google_pprof",
    importpath = "github.com/google/pprof",
    sum = "h1:Ak8CrdlwwXwAZxzS66vgPt4U8yUZX7JwLvVR58FN5jM=",
    version = "v0.0.0-20200708004538-1a94d8640e99",
)

go_repository(
    name = "com_github_google_renameio",
    importpath = "github.com/google/renameio",
    sum = "h1:GOZbcHa3HfsPKPlmyPyN2KEohoMXOhdMbHrvbpl2QaA=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_google_uuid",
    importpath = "github.com/google/uuid",
    sum = "h1:EVhdT+1Kseyi1/pUmXKaFxYsDNy9RQYkMWRH68J/W7Y=",
    version = "v1.1.2",
)

go_repository(
    name = "com_github_googleapis_gax_go_v2",
    importpath = "github.com/googleapis/gax-go/v2",
    sum = "h1:sjZBwGj9Jlw33ImPtvFviGYvseOtDM7hkSKB7+Tv3SM=",
    version = "v2.0.5",
)

go_repository(
    name = "com_github_googleapis_gnostic",
    importpath = "github.com/googleapis/gnostic",
    sum = "h1:DLJCy1n/vrD4HPjOvYcT8aYQXpPIzoRZONaYwyycI+I=",
    version = "v0.4.1",
)

go_repository(
    name = "com_github_gopherjs_gopherjs",
    importpath = "github.com/gopherjs/gopherjs",
    sum = "h1:EGx4pi6eqNxGaHF6qqu48+N2wcFQ5qg5FXgOdqsJ5d8=",
    version = "v0.0.0-20181017120253-0766667cb4d1",
)

go_repository(
    name = "com_github_gorilla_context",
    importpath = "github.com/gorilla/context",
    sum = "h1:AWwleXJkX/nhcU9bZSnZoi3h/qGYqQAGhq6zZe/aQW8=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_gorilla_mux",
    importpath = "github.com/gorilla/mux",
    sum = "h1:VuZ8uybHlWmqV03+zRzdwKL4tUnIp1MAQtp1mIFE1bc=",
    version = "v1.7.4",
)

go_repository(
    name = "com_github_gorilla_websocket",
    importpath = "github.com/gorilla/websocket",
    sum = "h1:+/TMaTYc4QFitKJxsQ7Yye35DkWvkdLcvGKqM+x0Ufc=",
    version = "v1.4.2",
)

go_repository(
    name = "com_github_gregjones_httpcache",
    importpath = "github.com/gregjones/httpcache",
    sum = "h1:+ngKgrYPPJrOjhax5N+uePQ0Fh1Z7PheYoUI/0nzkPA=",
    version = "v0.0.0-20190611155906-901d90724c79",
)

go_repository(
    name = "com_github_grpc_ecosystem_go_grpc_middleware",
    importpath = "github.com/grpc-ecosystem/go-grpc-middleware",
    sum = "h1:z53tR0945TRRQO/fLEVPI6SMv7ZflF0TEaTAoU7tOzg=",
    version = "v1.0.1-0.20190118093823-f849b5445de4",
)

go_repository(
    name = "com_github_grpc_ecosystem_go_grpc_prometheus",
    importpath = "github.com/grpc-ecosystem/go-grpc-prometheus",
    sum = "h1:Ovs26xHkKqVztRpIrF/92BcuyuQ/YW4NSIpoGtfXNho=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_grpc_ecosystem_grpc_gateway",
    importpath = "github.com/grpc-ecosystem/grpc-gateway",
    sum = "h1:UImYN5qQ8tuGpGE16ZmjvcTtTw24zw1QAp/SlnNrZhI=",
    version = "v1.9.5",
)

go_repository(
    name = "com_github_grpc_ecosystem_grpc_health_probe",
    importpath = "github.com/grpc-ecosystem/grpc-health-probe",
    sum = "h1:UxmGBzaBcWDQuQh9E1iT1dWKQFbizZ+SpTd1EL4MSqs=",
    version = "v0.2.1-0.20181220223928-2bf0a5b182db",
)

go_repository(
    name = "com_github_hashicorp_consul_api",
    importpath = "github.com/hashicorp/consul/api",
    sum = "h1:HXNYlRkkM/t+Y/Yhxtwcy02dlYwIaoxzvxPnS+cqy78=",
    version = "v1.3.0",
)

go_repository(
    name = "com_github_hashicorp_consul_sdk",
    importpath = "github.com/hashicorp/consul/sdk",
    sum = "h1:UOxjlb4xVNF93jak1mzzoBatyFju9nrkxpVwIp/QqxQ=",
    version = "v0.3.0",
)

go_repository(
    name = "com_github_hashicorp_errwrap",
    importpath = "github.com/hashicorp/errwrap",
    sum = "h1:hLrqtEDnRye3+sgx6z4qVLNuviH3MR5aQ0ykNJa/UYA=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_hashicorp_go_cleanhttp",
    importpath = "github.com/hashicorp/go-cleanhttp",
    sum = "h1:dH3aiDG9Jvb5r5+bYHsikaOUIpcM0xvgMXVoDkXMzJM=",
    version = "v0.5.1",
)

go_repository(
    name = "com_github_hashicorp_go_immutable_radix",
    importpath = "github.com/hashicorp/go-immutable-radix",
    sum = "h1:AKDB1HM5PWEA7i4nhcpwOrO2byshxBjXVn/J/3+z5/0=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_hashicorp_go_msgpack",
    importpath = "github.com/hashicorp/go-msgpack",
    sum = "h1:zKjpN5BK/P5lMYrLmBHdBULWbJ0XpYR+7NGzqkZzoD4=",
    version = "v0.5.3",
)

go_repository(
    name = "com_github_hashicorp_go_multierror",
    importpath = "github.com/hashicorp/go-multierror",
    sum = "h1:iVjPR7a6H0tWELX5NxNe7bYopibicUzc7uPribsnS6o=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_hashicorp_go_net",
    importpath = "github.com/hashicorp/go.net",
    sum = "h1:sNCoNyDEvN1xa+X0baata4RdcpKwcMS6DH+xwfqPgjw=",
    version = "v0.0.1",
)

go_repository(
    name = "com_github_hashicorp_go_rootcerts",
    importpath = "github.com/hashicorp/go-rootcerts",
    sum = "h1:Rqb66Oo1X/eSV1x66xbDccZjhJigjg0+e82kpwzSwCI=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_hashicorp_go_sockaddr",
    importpath = "github.com/hashicorp/go-sockaddr",
    sum = "h1:GeH6tui99pF4NJgfnhp+L6+FfobzVW3Ah46sLo0ICXs=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_hashicorp_go_syslog",
    importpath = "github.com/hashicorp/go-syslog",
    sum = "h1:KaodqZuhUoZereWVIYmpUgZysurB1kBLX2j0MwMrUAE=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_hashicorp_go_uuid",
    importpath = "github.com/hashicorp/go-uuid",
    sum = "h1:fv1ep09latC32wFoVwnqcnKJGnMSdBanPczbHAYm1BE=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_hashicorp_go_version",
    importpath = "github.com/hashicorp/go-version",
    sum = "h1:3vNe/fWF5CBgRIguda1meWhsZHy3m8gCJ5wx+dIzX/E=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_hashicorp_golang_lru",
    importpath = "github.com/hashicorp/golang-lru",
    sum = "h1:YDjusn29QI/Das2iO9M0BHnIbxPeyuCHsjMW+lJfyTc=",
    version = "v0.5.4",
)

go_repository(
    name = "com_github_hashicorp_hcl",
    importpath = "github.com/hashicorp/hcl",
    sum = "h1:0Anlzjpi4vEasTeNFn2mLJgTSwt0+6sfsiTG8qcWGx4=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_hashicorp_logutils",
    importpath = "github.com/hashicorp/logutils",
    sum = "h1:dLEQVugN8vlakKOUE3ihGLTZJRB4j+M2cdTm/ORI65Y=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_hashicorp_mdns",
    importpath = "github.com/hashicorp/mdns",
    sum = "h1:WhIgCr5a7AaVH6jPUwjtRuuE7/RDufnUvzIr48smyxs=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_hashicorp_memberlist",
    importpath = "github.com/hashicorp/memberlist",
    sum = "h1:EmmoJme1matNzb+hMpDuR/0sbJSUisxyqBGG676r31M=",
    version = "v0.1.3",
)

go_repository(
    name = "com_github_hashicorp_serf",
    importpath = "github.com/hashicorp/serf",
    sum = "h1:YZ7UKsJv+hKjqGVUUbtE3HNj79Eln2oQ75tniF6iPt0=",
    version = "v0.8.2",
)

go_repository(
    name = "com_github_hpcloud_tail",
    importpath = "github.com/hpcloud/tail",
    sum = "h1:nfCOvKYfkgYP8hkirhJocXT2+zOD8yUNjXaWfTlyFKI=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_hudl_fargo",
    importpath = "github.com/hudl/fargo",
    sum = "h1:0U6+BtN6LhaYuTnIJq4Wyq5cpn6O2kWrxAtcqBmYY6w=",
    version = "v1.3.0",
)

go_repository(
    name = "com_github_ianlancetaylor_demangle",
    importpath = "github.com/ianlancetaylor/demangle",
    sum = "h1:UDMh68UUwekSh5iP2OMhRRZJiiBccgV7axzUG8vi56c=",
    version = "v0.0.0-20181102032728-5e5cf60278f6",
)

go_repository(
    name = "com_github_imdario_mergo",
    importpath = "github.com/imdario/mergo",
    sum = "h1:3tnifQM4i+fbajXKBHXWEH+KvNHqojZ778UH75j3bGA=",
    version = "v0.3.11",
)

go_repository(
    name = "com_github_improbable_eng_thanos",
    importpath = "github.com/improbable-eng/thanos",
    sum = "h1:iZfU7exq+RD5Lnb8n3Eh9MNYoRLeyeGO/85AvEkLg+8=",
    version = "v0.3.2",
)

go_repository(
    name = "com_github_inconshreveable_mousetrap",
    importpath = "github.com/inconshreveable/mousetrap",
    sum = "h1:Z8tu5sraLXCXIcARxBp/8cbvlwVa7Z1NHg9XEKhtSvM=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_influxdata_influxdb1_client",
    importpath = "github.com/influxdata/influxdb1-client",
    sum = "h1:/WZQPMZNsjZ7IlCpsLGdQBINg5bxKQ1K1sh6awxLtkA=",
    version = "v0.0.0-20191209144304-8bf82d3c094d",
)

go_repository(
    name = "com_github_jmespath_go_jmespath",
    importpath = "github.com/jmespath/go-jmespath",
    sum = "h1:pmfjZENx5imkbgOkpRUYLnmbU7UEFbjtDA2hxJ1ichM=",
    version = "v0.0.0-20180206201540-c2b33e8439af",
)

go_repository(
    name = "com_github_joefitzgerald_rainbow_reporter",
    importpath = "github.com/joefitzgerald/rainbow-reporter",
    sum = "h1:AuMG652zjdzI0YCCnXAqATtRBpGXMcAnrajcaTrSeuo=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_jonboulle_clockwork",
    importpath = "github.com/jonboulle/clockwork",
    sum = "h1:VKV+ZcuP6l3yW9doeqz6ziZGgcynBVQO+obU0+0hcPo=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_jpillora_backoff",
    importpath = "github.com/jpillora/backoff",
    sum = "h1:uvFg412JmmHBHw7iwprIxkPMI+sGQ4kzOWsMeHnm2EA=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_json_iterator_go",
    importpath = "github.com/json-iterator/go",
    sum = "h1:uVUAXhF2To8cbw/3xN3pxj6kk7TYKs98NIrTqPlMWAQ=",
    version = "v1.1.11",
)

go_repository(
    name = "com_github_jsonnet_bundler_jsonnet_bundler",
    importpath = "github.com/jsonnet-bundler/jsonnet-bundler",
    sum = "h1:T/HtHFr+mYCRULrH1x/RnoB0prIs0rMkolJhFMXNC9A=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_jstemmer_go_junit_report",
    importpath = "github.com/jstemmer/go-junit-report",
    sum = "h1:6QPYqodiu3GuPL+7mfx+NwDdp2eTkp9IfEUpgAwUN0o=",
    version = "v0.9.1",
)

go_repository(
    name = "com_github_jtolds_gls",
    importpath = "github.com/jtolds/gls",
    sum = "h1:xdiiI2gbIgH/gLH7ADydsJ1uDOEzR8yvV7C0MuV77Wo=",
    version = "v4.20.0+incompatible",
)

go_repository(
    name = "com_github_julienschmidt_httprouter",
    importpath = "github.com/julienschmidt/httprouter",
    sum = "h1:U0609e9tgbseu3rBINet9P48AI/D3oJs4dN7jwJOQ1U=",
    version = "v1.3.0",
)

go_repository(
    name = "com_github_k8snetworkplumbingwg_network_attachment_definition_client",
    importpath = "github.com/k8snetworkplumbingwg/network-attachment-definition-client",
    sum = "h1:IwEFm6n6dvFAqpi3BtcTgnjwM/oj9hA30ZV7d4I0FGU=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_kelseyhightower_envconfig",
    importpath = "github.com/kelseyhightower/envconfig",
    sum = "h1:Im6hONhd3pLkfDFsbRgu68RDNkGF1r3dvMUtDTo2cv8=",
    version = "v1.4.0",
)

go_repository(
    name = "com_github_kisielk_errcheck",
    importpath = "github.com/kisielk/errcheck",
    sum = "h1:e8esj/e4R+SAOwFwN+n3zr0nYeCyeweozKfO23MvHzY=",
    version = "v1.5.0",
)

go_repository(
    name = "com_github_kisielk_gotool",
    importpath = "github.com/kisielk/gotool",
    sum = "h1:AV2c/EiW3KqPNT9ZKl07ehoAGi4C5/01Cfbblndcapg=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_klauspost_compress",
    importpath = "github.com/klauspost/compress",
    sum = "h1:eLeJ3dr/Y9+XRfJT4l+8ZjmtB5RPJhucH2HeCV5+IZY=",
    version = "v1.10.8",
)

go_repository(
    name = "com_github_klauspost_pgzip",
    importpath = "github.com/klauspost/pgzip",
    sum = "h1:TQ7CNpYKovDOmqzRHKxJh0BeaBI7UdQZYc6p7pMQh1A=",
    version = "v1.2.4",
)

go_repository(
    name = "com_github_knetic_govaluate",
    importpath = "github.com/Knetic/govaluate",
    sum = "h1:1G1pk05UrOh0NlF1oeaaix1x8XzrfjIDK47TY0Zehcw=",
    version = "v3.0.1-0.20171022003610-9aa49832a739+incompatible",
)

go_repository(
    name = "com_github_konsorten_go_windows_terminal_sequences",
    importpath = "github.com/konsorten/go-windows-terminal-sequences",
    sum = "h1:CE8S1cTafDpPvMhIxNJKvHsGVBgn1xWYf1NbHQhywc8=",
    version = "v1.0.3",
)

go_repository(
    name = "com_github_konveyor_controller",
    importpath = "github.com/konveyor/controller",
    sum = "h1:It0jfQEyth6AC5z8f4oTmd4RaRsTlKubKfLh/TlLYqk=",
    version = "v0.10.0",
)

go_repository(
    name = "com_github_kr_logfmt",
    importpath = "github.com/kr/logfmt",
    sum = "h1:T+h1c/A9Gawja4Y9mFVWj2vyii2bbUNDw3kt9VxK2EY=",
    version = "v0.0.0-20140226030751-b84e30acd515",
)

go_repository(
    name = "com_github_kr_pretty",
    importpath = "github.com/kr/pretty",
    sum = "h1:s5hAObm+yFO5uHYt5dYjxi2rXrsnmRpJx4OYvIWUaQs=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_kr_pty",
    importpath = "github.com/kr/pty",
    sum = "h1:hyz3dwM5QLc1Rfoz4FuWJQG5BN7tc6K1MndAUnGpQr4=",
    version = "v1.1.5",
)

go_repository(
    name = "com_github_kr_text",
    importpath = "github.com/kr/text",
    sum = "h1:45sCR5RtlFHMR4UwH9sdQ5TC8v0qDQCHnXt+kaKSTVE=",
    version = "v0.1.0",
)

go_repository(
    name = "com_github_kubernetes_csi_csi_lib_utils",
    importpath = "github.com/kubernetes-csi/csi-lib-utils",
    sum = "h1:t1cS7HTD7z5D7h9iAdjWuHtMxJPb9s1fIv34rxytzqs=",
    version = "v0.7.0",
)

go_repository(
    name = "com_github_kubernetes_csi_csi_test",
    importpath = "github.com/kubernetes-csi/csi-test",
    sum = "h1:ia04uVFUM/J9n/v3LEMn3rEG6FmKV5BH9QLw7H68h44=",
    version = "v2.0.0+incompatible",
)

go_repository(
    name = "com_github_kubernetes_csi_external_snapshotter_v2",
    importpath = "github.com/kubernetes-csi/external-snapshotter/v2",
    sum = "h1:t5bmB3Y8nCaLA4aFrIpX0zjHEF/HUkJp6f5rm7BsVzM=",
    version = "v2.1.1",
)

go_repository(
    name = "com_github_kubernetes_sigs_kube_storage_version_migrator",
    importpath = "github.com/kubernetes-sigs/kube-storage-version-migrator",
    sum = "h1:XZfEWaxPMR/cRvQ/SFWOjok7YEnURVYrCiIltx/0HGY=",
    version = "v0.0.0-20191127225502-51849bc15f17",
)

go_repository(
    name = "com_github_kylelemons_godebug",
    importpath = "github.com/kylelemons/godebug",
    sum = "h1:MtvEpTB6LX3vkb4ax0b5D2DHbNAUsen0Gx5wZoq3lV4=",
    version = "v0.0.0-20170820004349-d65d576e9348",
)

go_repository(
    name = "com_github_leodido_go_urn",
    importpath = "github.com/leodido/go-urn",
    sum = "h1:hpXL4XnriNwQ/ABnpepYM/1vCLWNDfUNts8dX3xTG6Y=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_liggitt_tabwriter",
    importpath = "github.com/liggitt/tabwriter",
    sum = "h1:9TO3cAIGXtEhnIaL+V+BEER86oLrvS+kWobKpbJuye0=",
    version = "v0.0.0-20181228230101-89fcab3d43de",
)

go_repository(
    name = "com_github_lightstep_lightstep_tracer_common_golang_gogo",
    importpath = "github.com/lightstep/lightstep-tracer-common/golang/gogo",
    sum = "h1:143Bb8f8DuGWck/xpNUOckBVYfFbBTnLevfRZ1aVVqo=",
    version = "v0.0.0-20190605223551-bc2310a04743",
)

go_repository(
    name = "com_github_lightstep_lightstep_tracer_go",
    importpath = "github.com/lightstep/lightstep-tracer-go",
    sum = "h1:vi1F1IQ8N7hNWytK9DpJsUfQhGuNSc19z330K6vl4zk=",
    version = "v0.18.1",
)

go_repository(
    name = "com_github_lyft_protoc_gen_validate",
    importpath = "github.com/lyft/protoc-gen-validate",
    sum = "h1:KNt/RhmQTOLr7Aj8PsJ7mTronaFyx80mRTT9qF261dA=",
    version = "v0.0.13",
)

go_repository(
    name = "com_github_magiconair_properties",
    importpath = "github.com/magiconair/properties",
    sum = "h1:ZC2Vc7/ZFkGmsVC9KvOjumD+G5lXy2RtTKyzRKO2BQ4=",
    version = "v1.8.1",
)

go_repository(
    name = "com_github_mailru_easyjson",
    importpath = "github.com/mailru/easyjson",
    sum = "h1:aizVhC/NAAcKWb+5QsU1iNOZb4Yws5UO2I+aIprQITM=",
    version = "v0.7.0",
)

go_repository(
    name = "com_github_mattn_go_colorable",
    importpath = "github.com/mattn/go-colorable",
    sum = "h1:c1ghPdyEDarC70ftn0y+A/Ee++9zz8ljHG1b13eJ0s8=",
    version = "v0.1.8",
)

go_repository(
    name = "com_github_mattn_go_isatty",
    importpath = "github.com/mattn/go-isatty",
    sum = "h1:wuysRhFDzyxgEmMf5xjvJ2M9dZoWAXNNr5LSBS7uHXY=",
    version = "v0.0.12",
)

go_repository(
    name = "com_github_mattn_go_runewidth",
    importpath = "github.com/mattn/go-runewidth",
    sum = "h1:Lm995f3rfxdpd6TSmuVCHVb/QhupuXlYr8sCI/QdE+0=",
    version = "v0.0.9",
)

go_repository(
    name = "com_github_mattn_go_shellwords",
    importpath = "github.com/mattn/go-shellwords",
    sum = "h1:Y7Xqm8piKOO3v10Thp7Z36h4FYFjt5xB//6XvOrs2Gw=",
    version = "v1.0.10",
)

go_repository(
    name = "com_github_mattn_go_sqlite3",
    importpath = "github.com/mattn/go-sqlite3",
    sum = "h1:4rQjbDxdu9fSgI/r3KN72G3c2goxknAqHHgPWWs8UlI=",
    version = "v1.14.4",
)

go_repository(
    name = "com_github_matttproud_golang_protobuf_extensions",
    importpath = "github.com/matttproud/golang_protobuf_extensions",
    sum = "h1:I0XW9+e1XWDxdcEniV4rQAIOPUGDq67JSCiRCgGCZLI=",
    version = "v1.0.2-0.20181231171920-c182affec369",
)

go_repository(
    name = "com_github_maxbrunsfeld_counterfeiter",
    importpath = "github.com/maxbrunsfeld/counterfeiter",
    sum = "h1:fJasMUaV/LYZvzK4bUOj13rNXc4fhVzU0Vu1OlcGUd4=",
    version = "v0.0.0-20181017030959-1aadac120687",
)

go_repository(
    name = "com_github_maxbrunsfeld_counterfeiter_v6",
    importpath = "github.com/maxbrunsfeld/counterfeiter/v6",
    sum = "h1:s0HwWQiNYF+YpoOncE8OxHVYG3YShNiRG8iuPDiSDWM=",
    version = "v6.2.1",
)

go_repository(
    name = "com_github_microsoft_go_winio",
    importpath = "github.com/Microsoft/go-winio",
    sum = "h1:ygIc8M6trr62pF5DucadTWGdEB4mEyvzi0e2nbcmcyA=",
    version = "v0.4.15-0.20190919025122-fc70bd9a86b5",
)

go_repository(
    name = "com_github_microsoft_hcsshim",
    importpath = "github.com/Microsoft/hcsshim",
    sum = "h1:VrfodqvztU8YSOvygU+DN1BGaSGxmrNfqOv5oOuX2Bk=",
    version = "v0.8.9",
)

go_repository(
    name = "com_github_miekg_dns",
    importpath = "github.com/miekg/dns",
    sum = "h1:9jZdLNd/P4+SfEJ0TNyxYpsK8N4GtfylBLqtbYN1sbA=",
    version = "v1.0.14",
)

go_repository(
    name = "com_github_mistifyio_go_zfs",
    importpath = "github.com/mistifyio/go-zfs",
    sum = "h1:gAMO1HM9xBRONLHHYnu5iFsOJUiJdNZo6oqSENd4eW8=",
    version = "v2.1.1+incompatible",
)

go_repository(
    name = "com_github_mitchellh_cli",
    importpath = "github.com/mitchellh/cli",
    sum = "h1:iGBIsUe3+HZ/AD/Vd7DErOt5sU9fa8Uj7A2s1aggv1Y=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_mitchellh_go_homedir",
    importpath = "github.com/mitchellh/go-homedir",
    sum = "h1:lukF9ziXFxDFPkA1vsr5zpc1XuPDn/wFntq5mG+4E0Y=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_mitchellh_go_testing_interface",
    importpath = "github.com/mitchellh/go-testing-interface",
    sum = "h1:fzU/JVNcaqHQEcVFAKeR41fkiLdIPrefOvVG1VZ96U0=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_mitchellh_gox",
    importpath = "github.com/mitchellh/gox",
    sum = "h1:lfGJxY7ToLJQjHHwi0EX6uYBdK78egf954SQl13PQJc=",
    version = "v0.4.0",
)

go_repository(
    name = "com_github_mitchellh_hashstructure",
    importpath = "github.com/mitchellh/hashstructure",
    sum = "h1:ZkRJX1CyOoTkar7p/mLS5TZU4nJ1Rn/F8u9dGS02Q3Y=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_mitchellh_iochan",
    importpath = "github.com/mitchellh/iochan",
    sum = "h1:C+X3KsSTLFVBr/tK1eYN/vs4rJcvsiLU338UhYPJWeY=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_mitchellh_mapstructure",
    importpath = "github.com/mitchellh/mapstructure",
    sum = "h1:fmNYVwqnSfB9mZU6OS2O6GsXM+wcskZDuKQzvN1EDeE=",
    version = "v1.1.2",
)

go_repository(
    name = "com_github_moby_term",
    importpath = "github.com/moby/term",
    sum = "h1:aY7OQNf2XqY/JQ6qREWamhI/81os/agb2BAGpcx5yWI=",
    version = "v0.0.0-20200312100748-672ec06f55cd",
)

go_repository(
    name = "com_github_modern_go_concurrent",
    importpath = "github.com/modern-go/concurrent",
    sum = "h1:TRLaZ9cD/w8PVh93nsPXa1VrQ6jlwL5oN8l14QlcNfg=",
    version = "v0.0.0-20180306012644-bacd9c7ef1dd",
)

go_repository(
    name = "com_github_modern_go_reflect2",
    importpath = "github.com/modern-go/reflect2",
    sum = "h1:9f412s+6RmYXLWZSEzVVgPGK7C2PphHj5RJrvfx9AWI=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_morikuni_aec",
    importpath = "github.com/morikuni/aec",
    sum = "h1:nP9CBfwrvYnBRgY6qfDQkygYDmYwOilePFkwzv4dU8A=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_mrnold_go_libnbd",
    importpath = "github.com/mrnold/go-libnbd",
    sum = "h1:bIszEpQZKre4DMqEIO5HCv/MA44ujD2w7BIQg/cOFts=",
    version = "v1.4.1-cdi",
)

go_repository(
    name = "com_github_mtrmac_gpgme",
    importpath = "github.com/mtrmac/gpgme",
    sum = "h1:dNOmvYmsrakgW7LcgiprD0yfRuQQe8/C8F6Z+zogO3s=",
    version = "v0.1.2",
)

go_repository(
    name = "com_github_munnerz_goautoneg",
    importpath = "github.com/munnerz/goautoneg",
    sum = "h1:C3w9PqII01/Oq1c1nUAm88MOHcQC9l5mIlSMApZMrHA=",
    version = "v0.0.0-20191010083416-a7dc8b61c822",
)

go_repository(
    name = "com_github_mwitkow_go_conntrack",
    importpath = "github.com/mwitkow/go-conntrack",
    sum = "h1:KUppIJq7/+SVif2QVs3tOP0zanoHgBEVAwHxUSIzRqU=",
    version = "v0.0.0-20190716064945-2f068394615f",
)

go_repository(
    name = "com_github_mxk_go_flowrate",
    importpath = "github.com/mxk/go-flowrate",
    sum = "h1:y5//uYreIhSUg3J1GEMiLbxo1LJaP8RfCpH6pymGZus=",
    version = "v0.0.0-20140419014527-cca7078d478f",
)

go_repository(
    name = "com_github_nats_io_jwt",
    importpath = "github.com/nats-io/jwt",
    sum = "h1:+RB5hMpXUUA2dfxuhBTEkMOrYmM+gKIZYS1KjSostMI=",
    version = "v0.3.2",
)

go_repository(
    name = "com_github_nats_io_nats_go",
    importpath = "github.com/nats-io/nats.go",
    sum = "h1:ik3HbLhZ0YABLto7iX80pZLPw/6dx3T+++MZJwLnMrQ=",
    version = "v1.9.1",
)

go_repository(
    name = "com_github_nats_io_nats_server_v2",
    importpath = "github.com/nats-io/nats-server/v2",
    sum = "h1:i2Ly0B+1+rzNZHHWtD4ZwKi+OU5l+uQo1iDHZ2PmiIc=",
    version = "v2.1.2",
)

go_repository(
    name = "com_github_nats_io_nkeys",
    importpath = "github.com/nats-io/nkeys",
    sum = "h1:6JrEfig+HzTH85yxzhSVbjHRJv9cn0p6n3IngIcM5/k=",
    version = "v0.1.3",
)

go_repository(
    name = "com_github_nats_io_nuid",
    importpath = "github.com/nats-io/nuid",
    sum = "h1:5iA8DT8V7q8WK2EScv2padNa/rTESc1KdnPw4TC2paw=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_nxadm_tail",
    importpath = "github.com/nxadm/tail",
    sum = "h1:DQuhQpB1tVlglWS2hLQ5OV6B5r8aGxSrPc5Qo6uTN78=",
    version = "v1.4.4",
)

go_repository(
    name = "com_github_nytimes_gziphandler",
    importpath = "github.com/NYTimes/gziphandler",
    sum = "h1:iLrQrdwjDd52kHDA5op2UBJFjmOb9g+7scBan4RN8F0=",
    version = "v1.0.1",
)

go_repository(
    name = "com_github_oklog_oklog",
    importpath = "github.com/oklog/oklog",
    sum = "h1:wVfs8F+in6nTBMkA7CbRw+zZMIB7nNM825cM1wuzoTk=",
    version = "v0.3.2",
)

go_repository(
    name = "com_github_oklog_run",
    importpath = "github.com/oklog/run",
    sum = "h1:Ru7dDtJNOyC66gQ5dQmaCa0qIsAUFY3sFpK1Xk8igrw=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_oklog_ulid",
    importpath = "github.com/oklog/ulid",
    sum = "h1:EGfNDEx6MqHz8B3uNV6QAib1UR2Lm97sHi3ocA6ESJ4=",
    version = "v1.3.1",
)

go_repository(
    name = "com_github_olekukonko_tablewriter",
    importpath = "github.com/olekukonko/tablewriter",
    sum = "h1:58+kh9C6jJVXYjt8IE48G2eWl6BjwU5Gj0gqY84fy78=",
    version = "v0.0.0-20170122224234-a0225b3f23b5",
)

go_repository(
    name = "com_github_oneofone_xxhash",
    importpath = "github.com/OneOfOne/xxhash",
    sum = "h1:KMrpdQIwFcEqXDklaen+P1axHaj9BSKzvpUUfnHldSE=",
    version = "v1.2.2",
)

go_repository(
    name = "com_github_onsi_ginkgo",
    importpath = "github.com/onsi/ginkgo",
    sum = "h1:jMU0WaQrP0a/YAEq8eJmJKjBoMs+pClEr1vDMlM/Do4=",
    version = "v1.14.1",
)

go_repository(
    name = "com_github_onsi_gomega",
    importpath = "github.com/onsi/gomega",
    sum = "h1:gph6h/qe9GSUw1NhH1gp+qb+h8rXD8Cy60Z32Qw3ELA=",
    version = "v1.10.3",
)

go_repository(
    name = "com_github_op_go_logging",
    importpath = "github.com/op/go-logging",
    sum = "h1:lDH9UUVJtmYCjyT0CI4q8xvlXPxeZ0gYCVvWbmPlp88=",
    version = "v0.0.0-20160315200505-970db520ece7",
)

go_repository(
    name = "com_github_opencontainers_go_digest",
    importpath = "github.com/opencontainers/go-digest",
    sum = "h1:apOUWs51W5PlhuyGyz9FCeeBIOUDA/6nW8Oi/yOhh5U=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_opencontainers_image_spec",
    importpath = "github.com/opencontainers/image-spec",
    replace = "github.com/opencontainers/image-spec",
    sum = "h1:9yCKha/T5XdGtO0q9Q9a6T5NUCsTn/DrBg0D7ufOcFM=",
    version = "v1.0.2",
)

go_repository(
    name = "com_github_opencontainers_runc",
    importpath = "github.com/opencontainers/runc",
    sum = "h1:4+xo8mtWixbHoEm451+WJNUrq12o2/tDsyK9Vgc/NcA=",
    version = "v1.0.0-rc90",
)

go_repository(
    name = "com_github_opencontainers_runtime_spec",
    importpath = "github.com/opencontainers/runtime-spec",
    sum = "h1:eNUVfm/RFLIi1G7flU5/ZRTHvd4kcVuzfRnL6OFlzCI=",
    version = "v0.1.2-0.20190507144316-5b71a03e2700",
)

go_repository(
    name = "com_github_opencontainers_selinux",
    importpath = "github.com/opencontainers/selinux",
    sum = "h1:F6DgIsjgBIcDksLW4D5RG9bXok6oqZ3nvMwj4ZoFu/Q=",
    version = "v1.5.2",
)

go_repository(
    name = "com_github_openshift_api",
    importpath = "github.com/openshift/api",
    replace = "github.com/openshift/api",
    sum = "h1:/h7PRJsGBUyre9lQQwmpjuap/x1K2bKgMmyRnCMsUEw=",
    version = "v0.0.0-20190716152234-9ea19f9dd578",
)

go_repository(
    name = "com_github_openshift_build_machinery_go",
    importpath = "github.com/openshift/build-machinery-go",
    sum = "h1:iP7TOaN+tEVNUQ0ODEbN1ukjLz918lsIt7Czf8giWlM=",
    version = "v0.0.0-20200713135615-1f43d26dccc7",
)

go_repository(
    name = "com_github_openshift_client_go",
    importpath = "github.com/openshift/client-go",
    replace = "github.com/openshift/client-go",
    sum = "h1:Otk3CuCAEHiMUr4Er6b+csq4Ar6qilAs9h93tbea+qM=",
    version = "v0.0.0-20191125132246-f6563a70e19a",
)

go_repository(
    name = "com_github_openshift_custom_resource_status",
    importpath = "github.com/openshift/custom-resource-status",
    sum = "h1:F1MEnOMwSrTA0YAkO0he9ip9w0JhYzI/iCB2mXmaSPg=",
    version = "v0.0.0-20200602122900-c002fd1547ca",
)

go_repository(
    name = "com_github_openshift_library_go",
    importpath = "github.com/openshift/library-go",
    sum = "h1:qzTuJSAJX5UMDW5oTb+RQTG6c2eP02bIjFgeQLL/W8o=",
    version = "v0.0.0-20200821154433-215f00df72cc",
)

go_repository(
    name = "com_github_openshift_prom_label_proxy",
    importpath = "github.com/openshift/prom-label-proxy",
    sum = "h1:GW8OxGwBbI2kCqjb5PQfVXRAuCJbYyX1RYs9R3ISjck=",
    version = "v0.1.1-0.20191016113035-b8153a7f39f1",
)

go_repository(
    name = "com_github_opentracing_basictracer_go",
    importpath = "github.com/opentracing/basictracer-go",
    sum = "h1:YyUAhaEfjoWXclZVJ9sGoNct7j4TVk7lZWlQw5UXuoo=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_opentracing_contrib_go_observer",
    importpath = "github.com/opentracing-contrib/go-observer",
    sum = "h1:lM6RxxfUMrYL/f8bWEUqdXrANWtrL7Nndbm9iFN0DlU=",
    version = "v0.0.0-20170622124052-a52f23424492",
)

go_repository(
    name = "com_github_opentracing_opentracing_go",
    importpath = "github.com/opentracing/opentracing-go",
    sum = "h1:pWlfV3Bxv7k65HYwkikxat0+s3pV4bsqf19k25Ur8rU=",
    version = "v1.1.0",
)

go_repository(
    name = "com_github_openzipkin_contrib_zipkin_go_opentracing",
    importpath = "github.com/openzipkin-contrib/zipkin-go-opentracing",
    sum = "h1:ZCnq+JUrvXcDVhX/xRolRBZifmabN1HcS1wrPSvxhrU=",
    version = "v0.4.5",
)

go_repository(
    name = "com_github_openzipkin_zipkin_go",
    importpath = "github.com/openzipkin/zipkin-go",
    sum = "h1:nY8Hti+WKaP0cRsSeQ026wU03QsM762XBeCXBb9NAWI=",
    version = "v0.2.2",
)

go_repository(
    name = "com_github_operator_framework_go_appr",
    importpath = "github.com/operator-framework/go-appr",
    sum = "h1:c7gnBIMtxxenMKXZjeCuQaDfn7IGGmgh9laEGsOEeU4=",
    version = "v0.0.0-20180917210448-f2aef88446f2",
)

go_repository(
    name = "com_github_operator_framework_operator_lifecycle_manager",
    importpath = "github.com/operator-framework/operator-lifecycle-manager",
    sum = "h1:500FBy57oogxOEDRQpDtVD1E5ZzmWh2n2iliXZv169s=",
    version = "v0.0.0-20190725173916-b56e63a643cc",
)

go_repository(
    name = "com_github_operator_framework_operator_marketplace",
    importpath = "github.com/operator-framework/operator-marketplace",
    sum = "h1:47MQUQRBZqwyTPLEHoFlbGRv63p0OvxpPp5g6FUQXQs=",
    version = "v0.0.0-20190216021216-57300a3ef3ba",
)

go_repository(
    name = "com_github_operator_framework_operator_registry",
    importpath = "github.com/operator-framework/operator-registry",
    sum = "h1:oDIevJvKXFsp7BEb7iJHuLvuhPZYBtIx5oZQ7iSISAs=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_ostreedev_ostree_go",
    importpath = "github.com/ostreedev/ostree-go",
    sum = "h1:TnbXhKzrTOyuvWrjI8W6pcoI9XPbLHFXCdN2dtUw7Rw=",
    version = "v0.0.0-20190702140239-759a8c1ac913",
)

go_repository(
    name = "com_github_ovirt_go_ovirt",
    importpath = "github.com/ovirt/go-ovirt",
    replace = "github.com/ovirt/go-ovirt",
    sum = "h1:jwvYN2BLH2JoOqX5ueldlcgwHsPdt2wx+crJSrYQ84A=",
    version = "v0.0.0-20210423075620-0fe653f1c0cd",
)

go_repository(
    name = "com_github_pact_foundation_pact_go",
    importpath = "github.com/pact-foundation/pact-go",
    sum = "h1:OYkFijGHoZAYbOIb1LWXrwKQbMMRUv1oQ89blD2Mh2Q=",
    version = "v1.0.4",
)

go_repository(
    name = "com_github_pascaldekloe_goe",
    importpath = "github.com/pascaldekloe/goe",
    sum = "h1:Lgl0gzECD8GnQ5QCWA8o6BtfL6mDH5rQgM4/fX3avOs=",
    version = "v0.0.0-20180627143212-57f6aae5913c",
)

go_repository(
    name = "com_github_pborman_uuid",
    importpath = "github.com/pborman/uuid",
    sum = "h1:+ZZIw58t/ozdjRaXh/3awHfmWRbzYxJoAdNJxe/3pvw=",
    version = "v1.2.1",
)

go_repository(
    name = "com_github_pelletier_go_toml",
    importpath = "github.com/pelletier/go-toml",
    sum = "h1:T5zMGML61Wp+FlcbWjRDT7yAxhJNAiPPLOFECq181zc=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_performancecopilot_speed",
    importpath = "github.com/performancecopilot/speed",
    sum = "h1:2WnRzIquHa5QxaJKShDkLM+sc0JPuwhXzK8OYOyt3Vg=",
    version = "v3.0.0+incompatible",
)

go_repository(
    name = "com_github_petar_gollrb",
    importpath = "github.com/petar/GoLLRB",
    sum = "h1:AwcgVYzW1T+QuJ2fc55ceOSCiVaOpdYUNpFj9t7+n9U=",
    version = "v0.0.0-20130427215148-53be0d36a84c",
)

go_repository(
    name = "com_github_peterbourgon_diskv",
    importpath = "github.com/peterbourgon/diskv",
    sum = "h1:UBdAOUP5p4RWqPBg048CAvpKN+vxiaj6gdUUzhl4XmI=",
    version = "v2.0.1+incompatible",
)

go_repository(
    name = "com_github_pierrec_lz4",
    importpath = "github.com/pierrec/lz4",
    sum = "h1:2xWsjqPFWcplujydGg4WmhC/6fZqK42wMM8aXeqhl0I=",
    version = "v2.0.5+incompatible",
)

go_repository(
    name = "com_github_pkg_errors",
    importpath = "github.com/pkg/errors",
    sum = "h1:FEBLx1zS214owpjy7qsBeixbURkuhQAwrK5UwLGTwt4=",
    version = "v0.9.1",
)

go_repository(
    name = "com_github_pkg_profile",
    importpath = "github.com/pkg/profile",
    sum = "h1:OQIvuDgm00gWVWGTf4m4mCt6W1/0YqU7Ntg0mySWgaI=",
    version = "v1.3.0",
)

go_repository(
    name = "com_github_pmezard_go_difflib",
    importpath = "github.com/pmezard/go-difflib",
    sum = "h1:4DBwDE0NGyQoBHbLQYPwSUPoCMWR5BEzIk/f1lZbAQM=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_posener_complete",
    importpath = "github.com/posener/complete",
    sum = "h1:ccV59UEOTzVDnDUEFdT95ZzHVZ+5+158q8+SJb2QV5w=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_pquerna_cachecontrol",
    importpath = "github.com/pquerna/cachecontrol",
    sum = "h1:0XM1XL/OFFJjXsYXlG30spTkV/E9+gmd5GD1w2HE8xM=",
    version = "v0.0.0-20171018203845-0dec1b30a021",
)

go_repository(
    name = "com_github_pquerna_ffjson",
    importpath = "github.com/pquerna/ffjson",
    sum = "h1:kyf9snWXHvQc+yxE9imhdI8YAm4oKeZISlaAR+x73zs=",
    version = "v0.0.0-20190813045741-dac163c6c0a9",
)

go_repository(
    name = "com_github_prometheus_client_golang",
    importpath = "github.com/prometheus/client_golang",
    sum = "h1:HNkLOAEQMIDv/K+04rukrLx6ch7msSRwf3/SASFAGtQ=",
    version = "v1.11.0",
)

go_repository(
    name = "com_github_prometheus_client_model",
    importpath = "github.com/prometheus/client_model",
    sum = "h1:uq5h0d+GuxiXLJLNABMgp2qUWDPiLvgCzz2dUR+/W/M=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_prometheus_common",
    importpath = "github.com/prometheus/common",
    sum = "h1:iMAkS2TDoNWnKM+Kopnx/8tnEStIfpYA0ur0xQzzhMQ=",
    version = "v0.26.0",
)

go_repository(
    name = "com_github_prometheus_procfs",
    importpath = "github.com/prometheus/procfs",
    sum = "h1:mxy4L2jP6qMonqmq+aTtOx1ifVWUgG/TAmntgbh3xv4=",
    version = "v0.6.0",
)

go_repository(
    name = "com_github_prometheus_prometheus",
    importpath = "github.com/prometheus/prometheus",
    sum = "h1:EekL1S9WPoPtJL2NZvL+xo38iMpraOnyEHOiyZygMDY=",
    version = "v2.3.2+incompatible",
)

go_repository(
    name = "com_github_prometheus_tsdb",
    importpath = "github.com/prometheus/tsdb",
    sum = "h1:w1tAGxsBMLkuGrFMhqgcCeBkM5d1YI24udArs+aASuQ=",
    version = "v0.8.0",
)

go_repository(
    name = "com_github_puerkitobio_purell",
    importpath = "github.com/PuerkitoBio/purell",
    sum = "h1:WEQqlqaGbrPkxLJWfBwQmfEAE1Z7ONdDLqrN38tNFfI=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_puerkitobio_urlesc",
    importpath = "github.com/PuerkitoBio/urlesc",
    sum = "h1:d+Bc7a5rLufV/sSk/8dngufqelfh6jnri85riMAaF/M=",
    version = "v0.0.0-20170810143723-de5bf2ad4578",
)

go_repository(
    name = "com_github_rcrowley_go_metrics",
    importpath = "github.com/rcrowley/go-metrics",
    sum = "h1:9ZKAASQSHhDYGoxY8uLVpewe1GDZ2vu2Tr/vTdVAkFQ=",
    version = "v0.0.0-20181016184325-3113b8401b8a",
)

go_repository(
    name = "com_github_robfig_cron",
    importpath = "github.com/robfig/cron",
    sum = "h1:ZjScXvvxeQ63Dbyxy76Fj3AT3Ut0aKsyd2/tl3DTMuQ=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_rogpeppe_fastuuid",
    importpath = "github.com/rogpeppe/fastuuid",
    sum = "h1:gu+uRPtBe88sKxUCEXRoeCvVG90TJmwhiqRpvdhQFng=",
    version = "v0.0.0-20150106093220-6724a57986af",
)

go_repository(
    name = "com_github_rogpeppe_go_charset",
    importpath = "github.com/rogpeppe/go-charset",
    sum = "h1:BN/Nyn2nWMoqGRA7G7paDNDqTXE30mXGqzzybrfo05w=",
    version = "v0.0.0-20180617210344-2471d30d28b4",
)

go_repository(
    name = "com_github_rogpeppe_go_internal",
    importpath = "github.com/rogpeppe/go-internal",
    sum = "h1:RR9dF3JtopPvtkroDZuVD7qquD0bnHlKSqaQhgwt8yk=",
    version = "v1.3.0",
)

go_repository(
    name = "com_github_rs_cors",
    importpath = "github.com/rs/cors",
    sum = "h1:+88SsELBHx5r+hZ8TCkggzSstaWNbDvThkVK8H6f9ik=",
    version = "v1.7.0",
)

go_repository(
    name = "com_github_russross_blackfriday",
    importpath = "github.com/russross/blackfriday",
    sum = "h1:HyvC0ARfnZBqnXwABFeSZHpKvJHJJfPz81GNueLj0oo=",
    version = "v1.5.2",
)

go_repository(
    name = "com_github_russross_blackfriday_v2",
    importpath = "github.com/russross/blackfriday/v2",
    sum = "h1:lPqVAte+HuHNfhJ/0LC98ESWRz8afy9tM/0RK8m9o+Q=",
    version = "v2.0.1",
)

go_repository(
    name = "com_github_ryanuber_columnize",
    importpath = "github.com/ryanuber/columnize",
    sum = "h1:UFr9zpz4xgTnIE5yIMtWAMngCdZ9p/+q6lTbgelo80M=",
    version = "v0.0.0-20160712163229-9b3edd62028f",
)

go_repository(
    name = "com_github_samuel_go_zookeeper",
    importpath = "github.com/samuel/go-zookeeper",
    sum = "h1:p3Vo3i64TCLY7gIfzeQaUJ+kppEO5WQG3cL8iE8tGHU=",
    version = "v0.0.0-20190923202752-2cc03de413da",
)

go_repository(
    name = "com_github_sclevine_spec",
    importpath = "github.com/sclevine/spec",
    sum = "h1:1Jwdf9jSfDl9NVmt8ndHqbTZ7XCCPbh1jI3hkDBHVYA=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_sean_seed",
    importpath = "github.com/sean-/seed",
    sum = "h1:nn5Wsu0esKSJiIVhscUtVbo7ada43DJhG55ua/hjS5I=",
    version = "v0.0.0-20170313163322-e2103e2c3529",
)

go_repository(
    name = "com_github_sergi_go_diff",
    importpath = "github.com/sergi/go-diff",
    sum = "h1:Kpca3qRNrduNnOQeazBd0ysaKrUJiIuISHxogkT9RPQ=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_shopify_sarama",
    importpath = "github.com/Shopify/sarama",
    sum = "h1:9oksLxC6uxVPHPVYUmq6xhr1BOF/hHobWH2UzO67z1s=",
    version = "v1.19.0",
)

go_repository(
    name = "com_github_shopify_toxiproxy",
    importpath = "github.com/Shopify/toxiproxy",
    sum = "h1:TKdv8HiTLgE5wdJuEML90aBgNWsokNbMijUGhmcoBJc=",
    version = "v2.1.4+incompatible",
)

go_repository(
    name = "com_github_shurcool_sanitized_anchor_name",
    importpath = "github.com/shurcooL/sanitized_anchor_name",
    sum = "h1:PdmoCO6wvbs+7yrJyMORt4/BmY5IYyJwS/kOiWx8mHo=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_sirupsen_logrus",
    importpath = "github.com/sirupsen/logrus",
    sum = "h1:UBcNElsrwanuuMsnGSlYmtmgbb23qDR5dG+6X6Oo89I=",
    version = "v1.6.0",
)

go_repository(
    name = "com_github_smartystreets_assertions",
    importpath = "github.com/smartystreets/assertions",
    sum = "h1:zE9ykElWQ6/NYmHa3jpm/yHnI4xSofP+UP6SpjHcSeM=",
    version = "v0.0.0-20180927180507-b2de0cb4f26d",
)

go_repository(
    name = "com_github_smartystreets_goconvey",
    importpath = "github.com/smartystreets/goconvey",
    sum = "h1:fv0U8FUIMPNf1L9lnHLvLhgicrIVChEkdzIKYqbNC9s=",
    version = "v1.6.4",
)

go_repository(
    name = "com_github_soheilhy_cmux",
    importpath = "github.com/soheilhy/cmux",
    sum = "h1:0HKaf1o97UwFjHH9o5XsHUOF+tqmdA7KEzXLpiyaw0E=",
    version = "v0.1.4",
)

go_repository(
    name = "com_github_sony_gobreaker",
    importpath = "github.com/sony/gobreaker",
    sum = "h1:oMnRNZXX5j85zso6xCPRNPtmAycat+WcoKbklScLDgQ=",
    version = "v0.4.1",
)

go_repository(
    name = "com_github_spaolacci_murmur3",
    importpath = "github.com/spaolacci/murmur3",
    sum = "h1:qLC7fQah7D6K1B0ujays3HV9gkFtllcxhzImRR7ArPQ=",
    version = "v0.0.0-20180118202830-f09979ecbc72",
)

go_repository(
    name = "com_github_spf13_afero",
    importpath = "github.com/spf13/afero",
    sum = "h1:5jhuqJyZCZf2JRofRvN/nIFgIWNzPa3/Vz8mYylgbWc=",
    version = "v1.2.2",
)

go_repository(
    name = "com_github_spf13_cast",
    importpath = "github.com/spf13/cast",
    sum = "h1:oget//CVOEoFewqQxwr0Ej5yjygnqGkvggSE/gB35Q8=",
    version = "v1.3.0",
)

go_repository(
    name = "com_github_spf13_cobra",
    importpath = "github.com/spf13/cobra",
    sum = "h1:KfztREH0tPxJJ+geloSLaAkaPkr4ki2Er5quFV1TDo4=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_spf13_jwalterweatherman",
    importpath = "github.com/spf13/jwalterweatherman",
    sum = "h1:XHEdyB+EcvlqZamSM4ZOMGlc93t6AcsBEu9Gc1vn7yk=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_spf13_pflag",
    importpath = "github.com/spf13/pflag",
    sum = "h1:iy+VFUOCP1a+8yFto/drg2CJ5u0yRoB7fZw3DKv/JXA=",
    version = "v1.0.5",
)

go_repository(
    name = "com_github_spf13_viper",
    importpath = "github.com/spf13/viper",
    sum = "h1:xVKxvI7ouOI5I+U9s2eeiUfMaWBVoXA3AWskkrqK0VM=",
    version = "v1.7.0",
)

go_repository(
    name = "com_github_stevvooe_resumable",
    importpath = "github.com/stevvooe/resumable",
    sum = "h1:4bT0pPowCpQImewr+BjzfUKcuFW+KVyB8d1OF3b6oTI=",
    version = "v0.0.0-20180830230917-22b14a53ba50",
)

go_repository(
    name = "com_github_streadway_amqp",
    importpath = "github.com/streadway/amqp",
    sum = "h1:WhxRHzgeVGETMlmVfqhRn8RIeeNoPr2Czh33I4Zdccw=",
    version = "v0.0.0-20190827072141-edfb9018d271",
)

go_repository(
    name = "com_github_streadway_handy",
    importpath = "github.com/streadway/handy",
    sum = "h1:AhmOdSHeswKHBjhsLs/7+1voOxT+LLrSk/Nxvk35fug=",
    version = "v0.0.0-20190108123426-d5acb3125c2a",
)

go_repository(
    name = "com_github_stretchr_objx",
    importpath = "github.com/stretchr/objx",
    sum = "h1:Hbg2NidpLE8veEBkEZTL3CvlkUIVzuU9jDplZO54c48=",
    version = "v0.2.0",
)

go_repository(
    name = "com_github_stretchr_testify",
    importpath = "github.com/stretchr/testify",
    sum = "h1:hDPOHmpOpP40lSULcqw7IrRb/u7w6RpDC9399XyoNd0=",
    version = "v1.6.1",
)

go_repository(
    name = "com_github_subosito_gotenv",
    importpath = "github.com/subosito/gotenv",
    sum = "h1:Slr1R9HxAlEKefgq5jn9U+DnETlIUa6HfgEzj0g5d7s=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_syndtr_gocapability",
    importpath = "github.com/syndtr/gocapability",
    sum = "h1:b6uOv7YOFK0TYG7HtkIgExQo+2RdLuwRft63jn2HWj8=",
    version = "v0.0.0-20180916011248-d98352740cb2",
)

go_repository(
    name = "com_github_tchap_go_patricia",
    importpath = "github.com/tchap/go-patricia",
    sum = "h1:GkY4dP3cEfEASBPPkWd+AmjYxhmDkqO9/zg7R0lSQRs=",
    version = "v2.3.0+incompatible",
)

go_repository(
    name = "com_github_tidwall_pretty",
    importpath = "github.com/tidwall/pretty",
    sum = "h1:HsD+QiTn7sK6flMKIvNmpqz1qrpP3Ps6jOKIKMooyg4=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_tmc_grpc_websocket_proxy",
    importpath = "github.com/tmc/grpc-websocket-proxy",
    sum = "h1:LnC5Kc/wtumK+WB441p7ynQJzVuNRJiqddSIE3IlSEQ=",
    version = "v0.0.0-20190109142713-0ad062ec5ee5",
)

go_repository(
    name = "com_github_ugorji_go",
    importpath = "github.com/ugorji/go",
    sum = "h1:/68gy2h+1mWMrwZFeD1kQialdSzAb432dtpeJ42ovdo=",
    version = "v1.1.7",
)

go_repository(
    name = "com_github_ugorji_go_codec",
    importpath = "github.com/ugorji/go/codec",
    sum = "h1:2SvQaVZ1ouYrrKKwoSk2pzd4A9evlKJb9oTL+OaLUSs=",
    version = "v1.1.7",
)

go_repository(
    name = "com_github_ulikunitz_xz",
    importpath = "github.com/ulikunitz/xz",
    sum = "h1:YvTNdFzX6+W5m9msiYg/zpkSURPPtOlzbqYjrFn7Yt4=",
    version = "v0.5.7",
)

go_repository(
    name = "com_github_xi2_xz",
    importpath = "github.com/xi2/xz",
    sum = "h1:nIPpBwaJSVYIxUFsDv3M8ofmx9yWTog9BfvIu0q41lo=",
    version = "v0.0.0-20171230120015-48954b6210f8",
)

go_repository(
    name = "com_github_urfave_cli",
    importpath = "github.com/urfave/cli",
    sum = "h1:+mkCCcOFKPnCmVYVcURKps1Xe+3zP90gSYGNfRkjoIY=",
    version = "v1.22.1",
)

go_repository(
    name = "com_github_vbatts_tar_split",
    importpath = "github.com/vbatts/tar-split",
    sum = "h1:0Odu65rhcZ3JZaPHxl7tCI3V/C/Q9Zf82UFravl02dE=",
    version = "v0.11.1",
)

go_repository(
    name = "com_github_vbauerster_mpb_v5",
    importpath = "github.com/vbauerster/mpb/v5",
    sum = "h1:zIICVOm+XD+uV6crpSORaL6I0Q1WqOdvxZTp+r3L9cw=",
    version = "v5.2.2",
)

go_repository(
    name = "com_github_vektah_gqlparser",
    importpath = "github.com/vektah/gqlparser",
    sum = "h1:ZsyLGn7/7jDNI+y4SEhI4yAxRChlv15pUHMjijT+e68=",
    version = "v1.1.2",
)

go_repository(
    name = "com_github_vishvananda_netlink",
    importpath = "github.com/vishvananda/netlink",
    sum = "h1:bqNY2lgheFIu1meHUFSH3d7vG93AFyqg3oGbJCOJgSM=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_vishvananda_netns",
    importpath = "github.com/vishvananda/netns",
    sum = "h1:OviZH7qLw/7ZovXvuNyL3XQl8UFofeikI1NW1Gypu7k=",
    version = "v0.0.0-20191106174202-0a2b9b5464df",
)

go_repository(
    name = "com_github_vividcortex_ewma",
    importpath = "github.com/VividCortex/ewma",
    sum = "h1:MnEK4VOv6n0RSY4vtRe3h11qjxL3+t0B8yOL8iMXdcM=",
    version = "v1.1.1",
)

go_repository(
    name = "com_github_vividcortex_gohistogram",
    importpath = "github.com/VividCortex/gohistogram",
    sum = "h1:6+hBz+qvs0JOrrNhhmR7lFxo5sINxBCGXrdtl/UvroE=",
    version = "v1.0.0",
)

go_repository(
    name = "com_github_vmware_govmomi",
    importpath = "github.com/vmware/govmomi",
    sum = "h1:vU09hxnNR/I7e+4zCJvW+5vHu5dO64Aoe2Lw7Yi/KRg=",
    version = "v0.23.1",
)

go_repository(
    name = "com_github_vmware_vmw_guestinfo",
    importpath = "github.com/vmware/vmw-guestinfo",
    sum = "h1:sH9mEk+flyDxiUa5BuPiuhDETMbzrt9A20I2wktMvRQ=",
    version = "v0.0.0-20170707015358-25eff159a728",
)

go_repository(
    name = "com_github_xeipuuv_gojsonpointer",
    importpath = "github.com/xeipuuv/gojsonpointer",
    sum = "h1:6cLsL+2FW6dRAdl5iMtHgRogVCff0QpRi9653YmdcJA=",
    version = "v0.0.0-20190809123943-df4f5c81cb3b",
)

go_repository(
    name = "com_github_xeipuuv_gojsonreference",
    importpath = "github.com/xeipuuv/gojsonreference",
    sum = "h1:EzJWgHovont7NscjpAxXsDA8S8BMYve8Y5+7cuRE7R0=",
    version = "v0.0.0-20180127040603-bd5ef7bd5415",
)

go_repository(
    name = "com_github_xeipuuv_gojsonschema",
    importpath = "github.com/xeipuuv/gojsonschema",
    sum = "h1:LhYJRs+L4fBtjZUfuSZIKGeVu0QRy8e5Xi7D17UxZ74=",
    version = "v1.2.0",
)

go_repository(
    name = "com_github_xiang90_probing",
    importpath = "github.com/xiang90/probing",
    sum = "h1:eY9dn8+vbi4tKz5Qo6v2eYzo7kUS51QINcR5jNpbZS8=",
    version = "v0.0.0-20190116061207-43a291ad63a2",
)

go_repository(
    name = "com_github_xlab_handysort",
    importpath = "github.com/xlab/handysort",
    sum = "h1:j2hhcujLRHAg872RWAV5yaUrEjHEObwDv3aImCaNLek=",
    version = "v0.0.0-20150421192137-fb3537ed64a1",
)

go_repository(
    name = "com_github_xordataexchange_crypt",
    importpath = "github.com/xordataexchange/crypt",
    sum = "h1:ESFSdwYZvkeru3RtdrYueztKhOBCSAAzS4Gf+k0tEow=",
    version = "v0.0.3-0.20170626215501-b2862e3d0a77",
)

go_repository(
    name = "com_github_yuin_goldmark",
    importpath = "github.com/yuin/goldmark",
    sum = "h1:ruQGxdhGHe7FWOJPT0mKs5+pD2Xs1Bm/kdGlHO04FmM=",
    version = "v1.2.1",
)

go_repository(
    name = "com_google_cloud_go",
    importpath = "cloud.google.com/go",
    sum = "h1:Dg9iHVQfrhq82rUNu9ZxUDrJLaxFUe/HlCVaLyRruq8=",
    version = "v0.65.0",
)

go_repository(
    name = "com_google_cloud_go_bigquery",
    importpath = "cloud.google.com/go/bigquery",
    sum = "h1:PQcPefKFdaIzjQFbiyOgAqyx8q5djaE7x9Sqe712DPA=",
    version = "v1.8.0",
)

go_repository(
    name = "com_google_cloud_go_datastore",
    importpath = "cloud.google.com/go/datastore",
    sum = "h1:/May9ojXjRkPBNVrq+oWLqmWCkr4OU5uRY29bu0mRyQ=",
    version = "v1.1.0",
)

go_repository(
    name = "com_google_cloud_go_firestore",
    importpath = "cloud.google.com/go/firestore",
    sum = "h1:9x7Bx0A9R5/M9jibeJeZWqjeVEIxYW9fZYqB9a70/bY=",
    version = "v1.1.0",
)

go_repository(
    name = "com_google_cloud_go_pubsub",
    importpath = "cloud.google.com/go/pubsub",
    sum = "h1:ukjixP1wl0LpnZ6LWtZJ0mX5tBmjp1f8Sqer8Z2OMUU=",
    version = "v1.3.1",
)

go_repository(
    name = "com_google_cloud_go_storage",
    importpath = "cloud.google.com/go/storage",
    sum = "h1:STgFzyU5/8miMl0//zKh2aQeTyeaUH3WN9bSUiJ09bA=",
    version = "v1.10.0",
)

go_repository(
    name = "com_shuralyov_dmitri_gpu_mtl",
    importpath = "dmitri.shuralyov.com/gpu/mtl",
    sum = "h1:VpgP7xuJadIUuKccphEpTJnWhS2jkQyMt6Y7pJCD7fY=",
    version = "v0.0.0-20190408044501-666a987793e9",
)

go_repository(
    name = "com_sourcegraph_sourcegraph_appdash",
    importpath = "sourcegraph.com/sourcegraph/appdash",
    sum = "h1:ucqkfpjg9WzSUubAO62csmucvxl4/JeW3F4I4909XkM=",
    version = "v0.0.0-20190731080439-ebfcffb1b5c0",
)

go_repository(
    name = "in_gopkg_alecthomas_kingpin_v2",
    importpath = "gopkg.in/alecthomas/kingpin.v2",
    sum = "h1:jMFz6MfLP0/4fUyZle81rXUoxOBFi19VUFKVDOQfozc=",
    version = "v2.2.6",
)

go_repository(
    name = "in_gopkg_asn1_ber_v1",
    importpath = "gopkg.in/asn1-ber.v1",
    sum = "h1:TxyelI5cVkbREznMhfzycHdkp5cLA7DpE+GKjSslYhM=",
    version = "v1.0.0-20181015200546-f715ec2f112d",
)

go_repository(
    name = "in_gopkg_check_v1",
    importpath = "gopkg.in/check.v1",
    sum = "h1:YR8cESwS4TdDjEe65xsg0ogRM/Nc3DYOhEAlW+xobZo=",
    version = "v1.0.0-20190902080502-41f04d3bba15",
)

go_repository(
    name = "in_gopkg_cheggaaa_pb_v1",
    importpath = "gopkg.in/cheggaaa/pb.v1",
    sum = "h1:Ev7yu1/f6+d+b3pi5vPdRPc6nNtP1umSfcWiEfRqv6I=",
    version = "v1.0.25",
)

go_repository(
    name = "in_gopkg_errgo_v2",
    importpath = "gopkg.in/errgo.v2",
    sum = "h1:0vLT13EuvQ0hNvakwLuFZ/jYrLp5F3kcWHXdRggjCE8=",
    version = "v2.1.0",
)

go_repository(
    name = "in_gopkg_fsnotify_v1",
    importpath = "gopkg.in/fsnotify.v1",
    sum = "h1:xOHLXZwVvI9hhs+cLKq5+I5onOuwQLhQwiu63xxlHs4=",
    version = "v1.4.7",
)

go_repository(
    name = "in_gopkg_gcfg_v1",
    importpath = "gopkg.in/gcfg.v1",
    sum = "h1:m8OOJ4ccYHnx2f4gQwpno8nAX5OGOh7RLaaz0pj3Ogs=",
    version = "v1.2.3",
)

go_repository(
    name = "in_gopkg_go_playground_assert_v1",
    importpath = "gopkg.in/go-playground/assert.v1",
    sum = "h1:xoYuJVE7KT85PYWrN730RguIQO0ePzVRfFMXadIrXTM=",
    version = "v1.2.1",
)

go_repository(
    name = "in_gopkg_go_playground_validator_v9",
    importpath = "gopkg.in/go-playground/validator.v9",
    sum = "h1:SvGtYmN60a5CVKTOzMSyfzWDeZRxRuGvRQyEAKbw1xc=",
    version = "v9.29.1",
)

go_repository(
    name = "in_gopkg_inf_v0",
    importpath = "gopkg.in/inf.v0",
    sum = "h1:73M5CoZyi3ZLMOyDlQh031Cx6N9NDJ2Vvfl76EDAgDc=",
    version = "v0.9.1",
)

go_repository(
    name = "in_gopkg_ini_v1",
    importpath = "gopkg.in/ini.v1",
    sum = "h1:AQvPpx3LzTDM0AjnIRlVFwFFGC+npRopjZxLJj6gdno=",
    version = "v1.51.0",
)

go_repository(
    name = "in_gopkg_ldap_v2",
    importpath = "gopkg.in/ldap.v2",
    sum = "h1:wiu0okdNfjlBzg6UWvd1Hn8Y+Ux17/u/4nlk4CQr6tU=",
    version = "v2.5.1",
)

go_repository(
    name = "in_gopkg_natefinch_lumberjack_v2",
    importpath = "gopkg.in/natefinch/lumberjack.v2",
    sum = "h1:1Lc07Kr7qY4U2YPouBjpCLxpiyxIVoxqXgkXLknAOE8=",
    version = "v2.0.0",
)

go_repository(
    name = "in_gopkg_resty_v1",
    importpath = "gopkg.in/resty.v1",
    sum = "h1:CuXP0Pjfw9rOuY6EP+UvtNvt5DSqHpIxILZKT/quCZI=",
    version = "v1.12.0",
)

go_repository(
    name = "in_gopkg_square_go_jose_v2",
    importpath = "gopkg.in/square/go-jose.v2",
    sum = "h1:SK5KegNXmKmqE342YYN2qPHEnUYeoMiXXl1poUlI+o4=",
    version = "v2.3.1",
)

go_repository(
    name = "in_gopkg_tomb_v1",
    importpath = "gopkg.in/tomb.v1",
    sum = "h1:uRGJdciOHaEIrze2W8Q3AKkepLTh2hOroT7a+7czfdQ=",
    version = "v1.0.0-20141024135613-dd632973f1e7",
)

go_repository(
    name = "in_gopkg_warnings_v0",
    importpath = "gopkg.in/warnings.v0",
    sum = "h1:wFXVbFY8DY5/xOe1ECiWdKCzZlxgshcYVNkBHstARME=",
    version = "v0.1.2",
)

go_repository(
    name = "in_gopkg_yaml_v2",
    importpath = "gopkg.in/yaml.v2",
    sum = "h1:clyUAQHOM3G0M3f5vQj7LuJrETvjVot3Z5el9nffUtU=",
    version = "v2.3.0",
)

go_repository(
    name = "in_gopkg_yaml_v3",
    importpath = "gopkg.in/yaml.v3",
    sum = "h1:tQIYjPdBoyREyB9XMu+nnTclpTYkz2zFM+lzLJFO4gQ=",
    version = "v3.0.0-20200615113413-eeeca48fe776",
)

go_repository(
    name = "io_etcd_go_bbolt",
    importpath = "go.etcd.io/bbolt",
    sum = "h1:XAzx9gjCb0Rxj7EoqcClPD1d5ZBxZJk0jbuoPHenBt0=",
    version = "v1.3.5",
)

go_repository(
    name = "io_etcd_go_etcd",
    importpath = "go.etcd.io/etcd",
    sum = "h1:Gqga3zA9tdAcfqobUGjSoCob5L3f8Dt5EuOp3ihNZko=",
    version = "v0.5.0-alpha.5.0.20200819165624-17cef6e3e9d5",
)

go_repository(
    name = "io_k8s_api",
    importpath = "k8s.io/api",
    replace = "k8s.io/api",
    sum = "h1:GN6ntFnv44Vptj/b+OnMW7FmzkpDoIDLZRvKX3XH9aU=",
    version = "v0.19.3",
)

go_repository(
    name = "io_k8s_apiextensions_apiserver",
    importpath = "k8s.io/apiextensions-apiserver",
    replace = "k8s.io/apiextensions-apiserver",
    sum = "h1:WZxBypSHW4SdXHbdPTS/Jy7L2la6Niggs8BuU5o+avo=",
    version = "v0.19.3",
)

go_repository(
    name = "io_k8s_apimachinery",
    importpath = "k8s.io/apimachinery",
    replace = "k8s.io/apimachinery",
    sum = "h1:bpIQXlKjB4cB/oNpnNnV+BybGPR7iP5oYpsOTEJ4hgc=",
    version = "v0.19.3",
)

go_repository(
    name = "io_k8s_apiserver",
    importpath = "k8s.io/apiserver",
    replace = "k8s.io/apiserver",
    sum = "h1:H7KUbLD74rh8NOPMLBJPSEG3Djqcv6Zxn5Ud0AL5u/k=",
    version = "v0.19.3",
)

go_repository(
    name = "io_k8s_client_go",
    importpath = "k8s.io/client-go",
    replace = "k8s.io/client-go",
    sum = "h1:ctqR1nQ52NUs6LpI0w+a5U+xjYwflFwA13OJKcicMxg=",
    version = "v0.19.3",
)

go_repository(
    name = "io_k8s_cluster_bootstrap",
    importpath = "k8s.io/cluster-bootstrap",
    replace = "k8s.io/cluster-bootstrap",
    sum = "h1:QY1qpB83m+mNlDcXLT+EpTGiUNVSNV0ybmgd/yZR5mA=",
    version = "v0.16.4",
)

go_repository(
    name = "io_k8s_code_generator",
    importpath = "k8s.io/code-generator",
    replace = "k8s.io/code-generator",
    sum = "h1:fTrTpJ8PZog5oo6MmeZtveo89emjQZHiw0ieybz1RSs=",
    version = "v0.19.3",
)

go_repository(
    name = "io_k8s_component_base",
    importpath = "k8s.io/component-base",
    replace = "k8s.io/component-base",
    sum = "h1:c+DzDNAQFlaoyX+yv8YuWi8xmlQvvY5DnJGbaz5U74o=",
    version = "v0.19.3",
)

go_repository(
    name = "io_k8s_gengo",
    importpath = "k8s.io/gengo",
    sum = "h1:t4L10Qfx/p7ASH3gXCdIUtPbbIuegCoUJf3TMSFekjw=",
    version = "v0.0.0-20200428234225-8167cfdcfc14",
)

go_repository(
    name = "io_k8s_klog",
    importpath = "k8s.io/klog",
    sum = "h1:Pt+yjF5aB1xDSVbau4VsWe+dQNzA0qv1LlXdC2dF6Q8=",
    version = "v1.0.0",
)

go_repository(
    name = "io_k8s_klog_v2",
    importpath = "k8s.io/klog/v2",
    sum = "h1:XRvcwJozkgZ1UQJmfMGpvRthQHOvihEhYtDfAaxMz/A=",
    version = "v2.2.0",
)

go_repository(
    name = "io_k8s_kube_aggregator",
    importpath = "k8s.io/kube-aggregator",
    replace = "k8s.io/kube-aggregator",
    sum = "h1:neDqyJ0tiP1RNhrS9Vk9o2Id/u5+TJX7BH0QBSkLYxc=",
    version = "v0.19.3",
)

go_repository(
    name = "io_k8s_kube_openapi",
    importpath = "k8s.io/kube-openapi",
    sum = "h1:sOHNzJIkytDF6qadMNKhhDRpc6ODik8lVC6nOur7B2c=",
    version = "v0.0.0-20201113171705-d219536bb9fd",
)

go_repository(
    name = "io_k8s_kubernetes",
    importpath = "k8s.io/kubernetes",
    replace = "k8s.io/kubernetes",
    sum = "h1:x6Q6M9nNBm9thoKj+PJr3HDPfHlz7FR35Ri61xpNfgg=",
    version = "v0.19.3",
)

go_repository(
    name = "io_k8s_sigs_apiserver_network_proxy_konnectivity_client",
    importpath = "sigs.k8s.io/apiserver-network-proxy/konnectivity-client",
    sum = "h1:rusRLrDhjBp6aYtl9sGEvQJr6faoHoDLd0YcUBTZguI=",
    version = "v0.0.9",
)

go_repository(
    name = "io_k8s_sigs_controller_runtime",
    importpath = "sigs.k8s.io/controller-runtime",
    replace = "sigs.k8s.io/controller-runtime",
    sum = "h1:4013CKsBs5bEqo+LevzDett+LLxag/FjQWG94nVZ/9g=",
    version = "v0.6.4",
)

go_repository(
    name = "io_k8s_sigs_controller_tools",
    importpath = "sigs.k8s.io/controller-tools",
    sum = "h1:3u2RCwOlp0cjCALAigpOcbAf50pE+kHSdueUosrC/AE=",
    version = "v0.5.0",
)

go_repository(
    name = "io_k8s_sigs_structured_merge_diff",
    importpath = "sigs.k8s.io/structured-merge-diff",
    replace = "sigs.k8s.io/structured-merge-diff",
    sum = "h1:zD2IemQ4LmOcAumeiyDWXKUI2SO0NYDe3H6QGvPOVgU=",
    version = "v1.0.1-0.20191108220359-b1b620dd3f06",
)

go_repository(
    name = "io_k8s_sigs_structured_merge_diff_v3",
    importpath = "sigs.k8s.io/structured-merge-diff/v3",
    sum = "h1:0KsuGbLhWdIxv5DA1OnbFz5hI/Co9kuxMfMUa5YsAHY=",
    version = "v3.0.0-20200116222232-67a7b8c61874",
)

go_repository(
    name = "io_k8s_sigs_structured_merge_diff_v4",
    importpath = "sigs.k8s.io/structured-merge-diff/v4",
    sum = "h1:YHQV7Dajm86OuqnIR6zAelnDWBRjo+YhYV9PmGrh1s8=",
    version = "v4.0.2",
)

go_repository(
    name = "io_k8s_sigs_testing_frameworks",
    importpath = "sigs.k8s.io/testing_frameworks",
    sum = "h1:vK0+tvjF0BZ/RYFeZ1E6BYBwHJJXhjuZ3TdsEKH+UQM=",
    version = "v0.1.2",
)

go_repository(
    name = "io_k8s_sigs_yaml",
    importpath = "sigs.k8s.io/yaml",
    sum = "h1:kr/MCeFWJWTwyaHoR9c8EjH9OumOmoF9YGiZd7lFm/Q=",
    version = "v1.2.0",
)

go_repository(
    name = "io_k8s_utils",
    importpath = "k8s.io/utils",
    sum = "h1:CbnUZsM497iRC5QMVkHwyl8s2tB3g7yaSHkYPkpgelw=",
    version = "v0.0.0-20201110183641-67b214c5f920",
)

go_repository(
    name = "io_kubevirt_client_go",
    importpath = "kubevirt.io/client-go",
    sum = "h1:Twh3JdWUptj1TIs5sI+jwDvcUXaCL2lsfF9PLZoCw7s=",
    version = "v0.42.1",
)

go_repository(
    name = "io_kubevirt_containerized_data_importer",
    importpath = "kubevirt.io/containerized-data-importer",
    sum = "h1:BrvtyeHdRWINSi+dQM46hxduUr4UwdEdwdYdX6okR14=",
    version = "v1.34.0",
)

go_repository(
    name = "io_kubevirt_containerized_data_importer_api",
    importpath = "kubevirt.io/containerized-data-importer-api",
    sum = "h1:SkqNC3Ccb3rv8vVvrwu/DPYK59cKOp7agC3WfSjwzPU=",
    version = "v1.44.0",
)

go_repository(
    name = "io_kubevirt_controller_lifecycle_operator_sdk",
    importpath = "kubevirt.io/controller-lifecycle-operator-sdk",
    sum = "h1:auv8LrA7gnLfQREnlGVPwgJpTxOEgnw4+mzXlUqKTxY=",
    version = "v0.2.3",
)

go_repository(
    name = "io_kubevirt_qe_tools",
    importpath = "kubevirt.io/qe-tools",
    sum = "h1:S6z9CATmgV2/z9CWetij++Rhu7l/Z4ObZqerLdNMo0Y=",
    version = "v0.1.6",
)

go_repository(
    name = "io_opencensus_go",
    importpath = "go.opencensus.io",
    sum = "h1:LYy1Hy3MJdrCdMwwzxA/dRok4ejH+RwNGbuoD9fCjto=",
    version = "v0.22.4",
)

go_repository(
    name = "io_rsc_binaryregexp",
    importpath = "rsc.io/binaryregexp",
    sum = "h1:HfqmD5MEmC0zvwBuF187nq9mdnXjXsSivRiXN7SmRkE=",
    version = "v0.2.0",
)

go_repository(
    name = "io_rsc_quote_v3",
    importpath = "rsc.io/quote/v3",
    sum = "h1:9JKUTTIUgS6kzR9mK1YuGKv6Nl+DijDNIc0ghT58FaY=",
    version = "v3.1.0",
)

go_repository(
    name = "io_rsc_sampler",
    importpath = "rsc.io/sampler",
    sum = "h1:7uVkIFmeBqHfdjD+gZwtXXI+RODJ2Wc4O7MPEh/QiW4=",
    version = "v1.3.0",
)

go_repository(
    name = "ml_vbom_util",
    importpath = "vbom.ml/util",
    sum = "h1:O69FD9pJA4WUZlEwYatBEEkRWKQ5cKodWpdKTrCS/iQ=",
    version = "v0.0.0-20180919145318-efcd4e0f9787",
)

go_repository(
    name = "org_bitbucket_ww_goautoneg",
    importpath = "bitbucket.org/ww/goautoneg",
    replace = "github.com/markusthoemmes/goautoneg",
    sum = "h1:Qhv4Ni88zV+8TY65yr2ak8xU4sblgs6aRT9RuGM5SNU=",
    version = "v0.0.0-20190713162725-c6008fefa5b1",
)

go_repository(
    name = "org_golang_google_api",
    importpath = "google.golang.org/api",
    sum = "h1:yfrXXP61wVuLb0vBcG6qaOoIoqYEzOQS8jum51jkv2w=",
    version = "v0.30.0",
)

go_repository(
    name = "org_golang_google_appengine",
    importpath = "google.golang.org/appengine",
    sum = "h1:lMO5rYAqUxkmaj76jAkRUvt5JZgFymx/+Q5Mzfivuhc=",
    version = "v1.6.6",
)

go_repository(
    name = "org_golang_google_genproto",
    importpath = "google.golang.org/genproto",
    sum = "h1:PDIOdWxZ8eRizhKa1AAvY53xsvLB1cWorMjslvY3VA8=",
    version = "v0.0.0-20200825200019-8632dd797987",
)

go_repository(
    name = "org_golang_google_grpc",
    importpath = "google.golang.org/grpc",
    sum = "h1:T7P4R73V3SSDPhH7WW7ATbfViLtmamH0DKrP3f9AuDI=",
    version = "v1.31.0",
)

go_repository(
    name = "org_golang_google_protobuf",
    importpath = "google.golang.org/protobuf",
    sum = "h1:7QnIQpGRHE5RnLKnESfDoxm2dTapTZua5a0kS0A+VXQ=",
    version = "v1.26.0-rc.1",
)

go_repository(
    name = "org_golang_x_crypto",
    importpath = "golang.org/x/crypto",
    sum = "h1:hb9wdF1z5waM+dSIICn1l0DkLVDT3hqhhQsDNUmHPRE=",
    version = "v0.0.0-20201002170205-7f63de1d35b0",
)

go_repository(
    name = "org_golang_x_exp",
    importpath = "golang.org/x/exp",
    sum = "h1:QE6XYQK6naiK1EPAe1g/ILLxN5RBoH5xkJk3CqlMI/Y=",
    version = "v0.0.0-20200224162631-6cc2880d07d6",
)

go_repository(
    name = "org_golang_x_image",
    importpath = "golang.org/x/image",
    sum = "h1:+qEpEAPhDZ1o0x3tHzZTQDArnOixOzGD9HUJfcg0mb4=",
    version = "v0.0.0-20190802002840-cff245a6509b",
)

go_repository(
    name = "org_golang_x_lint",
    importpath = "golang.org/x/lint",
    sum = "h1:Wh+f8QHJXR411sJR8/vRBTZ7YapZaRvUcLFFJhusH0k=",
    version = "v0.0.0-20200302205851-738671d3881b",
)

go_repository(
    name = "org_golang_x_mobile",
    importpath = "golang.org/x/mobile",
    sum = "h1:4+4C/Iv2U4fMZBiMCc98MG1In4gJY5YRhtpDNeDeHWs=",
    version = "v0.0.0-20190719004257-d2bd2a29d028",
)

go_repository(
    name = "org_golang_x_mod",
    importpath = "golang.org/x/mod",
    sum = "h1:RM4zey1++hCTbCVQfnWeKs9/IEsaBLA8vTkd0WVtmH4=",
    version = "v0.3.0",
)

go_repository(
    name = "org_golang_x_net",
    importpath = "golang.org/x/net",
    sum = "h1:uwuIcX0g4Yl1NC5XAz37xsr2lTtcqevgzYNVt49waME=",
    version = "v0.0.0-20201110031124-69a78807bb2b",
)

go_repository(
    name = "org_golang_x_oauth2",
    importpath = "golang.org/x/oauth2",
    sum = "h1:ld7aEMNHoBnnDAX15v1T6z31v8HwR2A9FYOuAhWqkwc=",
    version = "v0.0.0-20200902213428-5d25da1a8d43",
)

go_repository(
    name = "org_golang_x_sync",
    importpath = "golang.org/x/sync",
    sum = "h1:DcqTD9SDLc+1P/r1EmRBwnVsrOwW+kk2vWf9n+1sGhs=",
    version = "v0.0.0-20201207232520-09787c993a3a",
)

go_repository(
    name = "org_golang_x_sys",
    importpath = "golang.org/x/sys",
    sum = "h1:JWgyZ1qgdTaF3N3oxC+MdTV7qvEEgHo3otj+HB5CM7Q=",
    version = "v0.0.0-20210603081109-ebe580a85c40",
)

go_repository(
    name = "org_golang_x_text",
    importpath = "golang.org/x/text",
    sum = "h1:cokOdA+Jmi5PJGXLlLllQSgYigAEfHXJAERHVMaCc2k=",
    version = "v0.3.3",
)

go_repository(
    name = "org_golang_x_time",
    importpath = "golang.org/x/time",
    sum = "h1:/5xXl8Y5W96D+TtHSlonuFqGHIWVuyCkGJLwGh9JJFs=",
    version = "v0.0.0-20191024005414-555d28b269f0",
)

go_repository(
    name = "org_golang_x_tools",
    importpath = "golang.org/x/tools",
    sum = "h1:CB3a9Nez8M13wwlr/E2YtwoU+qYHKfC+JrDa45RXXoQ=",
    version = "v0.0.0-20210106214847-113979e3529a",
)

go_repository(
    name = "org_golang_x_xerrors",
    importpath = "golang.org/x/xerrors",
    sum = "h1:go1bK/D/BFZV2I8cIQd1NKEZ+0owSTG1fDTci4IqFcE=",
    version = "v0.0.0-20200804184101-5ec99f83aff1",
)

go_repository(
    name = "org_gonum_v1_gonum",
    importpath = "gonum.org/v1/gonum",
    sum = "h1:2qZ38BsejXrhuetzb8UxucqrWDZKjypFSZA82hLCpZ4=",
    version = "v0.0.0-20190710053202-4340aa3071a0",
)

go_repository(
    name = "org_gonum_v1_netlib",
    importpath = "gonum.org/v1/netlib",
    sum = "h1:OE9mWmgKkjJyEmDAAtGMPjXu+YNeGvK9VTSHY6+Qihc=",
    version = "v0.0.0-20190313105609-8cb42192e0e0",
)

go_repository(
    name = "org_libvirt_libvirt_go_xml",
    importpath = "libvirt.org/libvirt-go-xml",
    sum = "h1:OjqCuSgsfv+Ig3x8Xt7P8rBM2lNCA69eMaID45LeH48=",
    version = "v6.6.0+incompatible",
)

go_repository(
    name = "org_mongodb_go_mongo_driver",
    importpath = "go.mongodb.org/mongo-driver",
    sum = "h1:jxcFYjlkl8xaERsgLo+RNquI0epW6zuy/ZRQs6jnrFA=",
    version = "v1.1.2",
)

go_repository(
    name = "org_uber_go_atomic",
    importpath = "go.uber.org/atomic",
    sum = "h1:Ezj3JGmsOnG1MoRWQkPBsKLe9DwWD9QeXzTRzzldNVk=",
    version = "v1.6.0",
)

go_repository(
    name = "org_uber_go_multierr",
    importpath = "go.uber.org/multierr",
    sum = "h1:KCa4XfM8CWFCpxXRGok+Q0SS/0XBhMDbHHGABQLvD2A=",
    version = "v1.5.0",
)

go_repository(
    name = "org_uber_go_tools",
    importpath = "go.uber.org/tools",
    sum = "h1:0mgffUl7nfd+FpvXMVz4IDEaUSmT1ysygQC7qYo7sG4=",
    version = "v0.0.0-20190618225709-2cfd321de3ee",
)

go_repository(
    name = "org_uber_go_zap",
    importpath = "go.uber.org/zap",
    sum = "h1:nYDKopTbvAPq/NrUVZwT15y2lpROBiLLyoRTbXOYWOo=",
    version = "v1.14.1",
)

go_repository(
    name = "tools_gotest",
    importpath = "gotest.tools",
    sum = "h1:VsBPFP1AI068pPrMxtb/S8Zkgf9xEmTLJjfM+P5UIEo=",
    version = "v2.2.0+incompatible",
)

go_repository(
    name = "tools_gotest_v3",
    importpath = "gotest.tools/v3",
    sum = "h1:kG1BFyqVHuQoVQiR1bWGnfz/fmHvvuiSPIV7rvl360E=",
    version = "v3.0.2",
)

go_repository(
    name = "xyz_gomodules_jsonpatch_v2",
    importpath = "gomodules.xyz/jsonpatch/v2",
    sum = "h1:xyiBuvkD2g5n7cYzx6u2sxQvsAy4QJsZFCzGVdzOXZ0=",
    version = "v2.0.1",
)

go_rules_dependencies()

# NOTE: Keep the version in sync with Go toolchain in GitHub action.
go_register_toolchains(version = "1.19.3")

# override rules_docker issue with this dependency
# rules_docker 0.16 uses 0.1.4, bit since there the checksum changed, which is very weird, going with 0.1.4.1 to
go_repository(
    name = "com_github_google_go_containerregistry",
    importpath = "github.com/google/go-containerregistry",
    sha256 = "bc0136a33f9c1e4578a700f7afcdaa1241cfff997d6bba695c710d24c5ae26bd",
    strip_prefix = "google-go-containerregistry-efb2d62",
    type = "tar.gz",
    urls = ["https://api.github.com/repos/google/go-containerregistry/tarball/efb2d62d93a7705315b841d0544cb5b13565ff2a"],  # v0.1.4.1
)

# All dependencies defined with go_repository should be above
# gazelle_dependencies. Otherwise dependencies defined by Gazelle would be used
# because the first definitions of external repository wins.
gazelle_dependencies()

load(
    "@io_bazel_rules_docker//container:container.bzl",
    "container_pull",
)
load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

container_repositories()

container_pull(
    name = "ubi9-minimal",
    # 'tag' is also supported, but digest is encouraged for reproducibility.
    digest = "sha256:e9ea62ea2017705205ba7bc55d20827e06abe4fe071f0793c6cae46edd5855cf",
    registry = "registry.access.redhat.com",
    repository = "ubi9/ubi-minimal",
)

container_pull(
    name = "libguestfs-appliance",
    digest = "sha256:1c40f82eac823fc417dc69453685bb0cf79391e1306a2b576f88217d61abd644",
    registry = "quay.io",
    repository = "kubev2v/libguestfs-appliance",
)

container_pull(
    name = "ansible-operator-image",
    # v1.22.0
    digest = "sha256:e07ba82127e76f282cb61fad6cfd990ab137533e5e996686576dec088d5e7e44",
    registry = "quay.io",
    repository = "operator-framework/ansible-operator",
)

container_pull(
    name = "opm-image",
    digest = "sha256:601c62a5e3fea961665aad2ed2834f3f165a020051d355eb24af2125da8e158e",
    registry = "quay.io",
    repository = "operator-framework/opm",
)

http_file(
    name = "opa",
    downloaded_file_path = "opa",
    executable = True,
    sha256 = "5ddb21d3fcfca130a47a42e730c05f055c68af6c1b37465879f6c59b10527eae",
    urls = ["https://openpolicyagent.org/downloads/v0.44.0/opa_linux_amd64_static"],
)

http_file(
    name = "kustomize",
    downloaded_file_path = "kustomize.tar.gz",
    sha256 = "4a3372d7bfdffe2eaf729e77f88bc94ce37dc84de55616bfe90aac089bf6fd02",
    urls = [
        "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v3.8.7/kustomize_v3.8.7_linux_amd64.tar.gz",
    ],
)

http_file(
    name = "operator-sdk",
    downloaded_file_path = "operator-sdk",
    executable = True,
    sha256 = "2fc68a50b94b7c477e804729365baa5de6d5afcfea9b7fcac9f93dd649c29e90",
    urls = ["https://github.com/operator-framework/operator-sdk/releases/download/v1.22.0/operator-sdk_linux_amd64"],
)

http_file(
    name = "opm",
    downloaded_file_path = "opm",
    executable = True,
    sha256 = "dc0d4d287fef23f165c837b2e6cb68e2506ff295dc57110b9bfe3b553359eb36",
    urls = ["https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/linux-amd64-opm"],
)

http_archive(
    name = "bazeldnf",
    sha256 = "404fc34e6bd3b568a7ca6fbcde70267d43830d0171d3192e3ecd83c14c320cfc",
    strip_prefix = "bazeldnf-0.5.4",
    urls = [
        "https://github.com/rmohr/bazeldnf/archive/v0.5.4.tar.gz",
        "https://storage.googleapis.com/builddeps/404fc34e6bd3b568a7ca6fbcde70267d43830d0171d3192e3ecd83c14c320cfc",
    ],
)

load(
    "@io_bazel_rules_go//go:deps.bzl",
    "go_register_toolchains",
    "go_rules_dependencies",
)
load("@bazeldnf//:deps.bzl", "bazeldnf_dependencies", "rpm")

bazeldnf_dependencies()

http_archive(
    name = "rules_proto",
    sha256 = "bc12122a5ae4b517fa423ea03a8d82ea6352d5127ea48cb54bc324e8ab78493c",
    strip_prefix = "rules_proto-af6481970a34554c6942d993e194a9aed7987780",
    urls = [
        "https://github.com/bazelbuild/rules_proto/archive/af6481970a34554c6942d993e194a9aed7987780.tar.gz",
        "https://storage.googleapis.com/builddeps/bc12122a5ae4b517fa423ea03a8d82ea6352d5127ea48cb54bc324e8ab78493c",
    ],
)

load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies", "rules_proto_toolchains")

rules_proto_dependencies()

rules_proto_toolchains()

rpm(
    name = "acl-0__2.2.53-1.el8.x86_64",
    sha256 = "227de6071cd3aeca7e10ad386beaf38737d081e06350d02208a3f6a2c9710385",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/acl-2.2.53-1.el8.x86_64.rpm"],
)

rpm(
    name = "alsa-lib-0__1.2.8-2.el8.x86_64",
    sha256 = "85e359bfb9815fe72dc063790472758a3e8b2861814f695a116fa67fc8871a95",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/alsa-lib-1.2.8-2.el8.x86_64.rpm"],
)

rpm(
    name = "audit-libs-0__3.0.7-4.el8.x86_64",
    sha256 = "b37099679b46f9a15d20b7c54fdd993388a8b84105f76869494c1be17140b512",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/audit-libs-3.0.7-4.el8.x86_64.rpm"],
)

rpm(
    name = "augeas-libs-0__1.12.0-8.el8.x86_64",
    sha256 = "8d871c7339ed515b012497d8fe97bda5252649c14edfce27ade65ccd1edb16df",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/augeas-libs-1.12.0-8.el8.x86_64.rpm"],
)

rpm(
    name = "autogen-libopts-0__5.18.12-8.el8.x86_64",
    sha256 = "c73af033015bfbdbe8a43e162b098364d148517d394910f8db5d33b76b93aa48",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/autogen-libopts-5.18.12-8.el8.x86_64.rpm"],
)

rpm(
    name = "basesystem-0__11-5.el8.x86_64",
    sha256 = "48226934763e4c412c1eb65df314e6879720b4b1ebcb3d07c126c9526639cb68",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/basesystem-11-5.el8.noarch.rpm"],
)

rpm(
    name = "bash-0__4.4.20-4.el8.x86_64",
    sha256 = "a104837b8aea5214122cf09c2de436db8f528812c1361c39f2d7471343dc509b",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/bash-4.4.20-4.el8.x86_64.rpm"],
)

rpm(
    name = "bitmap-console-fonts-0__0.3-28.el8.x86_64",
    sha256 = "738d3664ceb31697b6bbba0aa12116d1444c0c2776fdf922ce6d90760b038875",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/bitmap-console-fonts-0.3-28.el8.noarch.rpm"],
)

rpm(
    name = "boost-atomic-0__1.66.0-13.el8.x86_64",
    sha256 = "582e24b683cbefbd6281036c177cab913e9bfe76f6a183caae1eff70983d2569",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/boost-atomic-1.66.0-13.el8.x86_64.rpm"],
)

rpm(
    name = "boost-chrono-0__1.66.0-13.el8.x86_64",
    sha256 = "2d676a5e03854931f9a71a9ab32261dee9540b7fdd6c70a5fddf69bcea818882",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/boost-chrono-1.66.0-13.el8.x86_64.rpm"],
)

rpm(
    name = "boost-date-time-0__1.66.0-13.el8.x86_64",
    sha256 = "34100778783c5748230b82cd259418a4d266fcfb2bcb6f30e7b854f7fed90c8f",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/boost-date-time-1.66.0-13.el8.x86_64.rpm"],
)

rpm(
    name = "boost-iostreams-0__1.66.0-13.el8.x86_64",
    sha256 = "5a85438daaf569dfba73e4708ce9987a84245ce797b2102a06f2043c96a31beb",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/boost-iostreams-1.66.0-13.el8.x86_64.rpm"],
)

rpm(
    name = "boost-program-options-0__1.66.0-13.el8.x86_64",
    sha256 = "015a3d3a9c7fba7b4ec16cf73512308f9b457410598a24c1a24c50ad8f2ef2a3",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/boost-program-options-1.66.0-13.el8.x86_64.rpm"],
)

rpm(
    name = "boost-random-0__1.66.0-13.el8.x86_64",
    sha256 = "e7991373724e31b0bc6ecd4208f509f9674cbe16f45e5ae50a6fdbd2e5456e57",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/boost-random-1.66.0-13.el8.x86_64.rpm"],
)

rpm(
    name = "boost-regex-0__1.66.0-13.el8.x86_64",
    sha256 = "185a1a5f4c642b14c7a700b4c757f962f4d959dd5a3018c44e43b10071081bb8",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/boost-regex-1.66.0-13.el8.x86_64.rpm"],
)

rpm(
    name = "boost-system-0__1.66.0-13.el8.x86_64",
    sha256 = "9bce2a6d122e4afedf305e6811d8db89046812f7e13203eb83ec608af65b3ba4",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/boost-system-1.66.0-13.el8.x86_64.rpm"],
)

rpm(
    name = "boost-thread-0__1.66.0-13.el8.x86_64",
    sha256 = "fa1a547d4bb6b481b74afb73833c81e91e8813056500464dbaef8c172d00be74",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/boost-thread-1.66.0-13.el8.x86_64.rpm"],
)

rpm(
    name = "bzip2-0__1.0.6-26.el8.x86_64",
    sha256 = "78596f457c3d737a97a4edfe9a03a01f593606379c281701ab7f7eba13ecaf18",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/bzip2-1.0.6-26.el8.x86_64.rpm"],
)

rpm(
    name = "bzip2-libs-0__1.0.6-26.el8.x86_64",
    sha256 = "19d66d152b745dbd49cea9d21c52aec0ec4d4321edef97a342acd3542404fa31",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/bzip2-libs-1.0.6-26.el8.x86_64.rpm"],
)

rpm(
    name = "ca-certificates-0__2022.2.54-80.2.el8.x86_64",
    sha256 = "3200d42d5585afa93a94600614a82b6e804139b06fff151576a53effd221e12b",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/ca-certificates-2022.2.54-80.2.el8.noarch.rpm"],
)

rpm(
    name = "cairo-0__1.15.12-6.el8.x86_64",
    sha256 = "8d94b1b954d06a5443c4e8036c1d51121a6724c1508f37539bbff96dbf806a92",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/cairo-1.15.12-6.el8.x86_64.rpm"],
)

rpm(
    name = "celt051-0__0.5.1.3-15.el8.x86_64",
    sha256 = "f689f4c20fb5de0e9c39b9c5f81e44fe89833aead1597de6454c2b459a2d1742",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/celt051-0.5.1.3-15.el8.x86_64.rpm"],
)

rpm(
    name = "centos-gpg-keys-1__8-6.el8.x86_64",
    sha256 = "567dd699e703dc6f5fa6ddb5548bf0dbd3bda08a0a6b1d10b32fa19012409cd0",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/centos-gpg-keys-8-6.el8.noarch.rpm"],
)

rpm(
    name = "centos-stream-release-0__8.6-1.el8.x86_64",
    sha256 = "3b3b86cb51f62632995ace850fbed9efc65381d639f1e1c5ceeff7ccf2dd6151",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/centos-stream-release-8.6-1.el8.noarch.rpm"],
)

rpm(
    name = "centos-stream-repos-0__8-6.el8.x86_64",
    sha256 = "ff0a2d1fb5b00e9a26b05a82675d0dcdf0378ee5476f9ae765b32399c2ee561f",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/centos-stream-repos-8-6.el8.noarch.rpm"],
)

rpm(
    name = "checkpolicy-0__2.9-1.el8.x86_64",
    sha256 = "d5c283da0d2666742635754626263f6f78e273cd46d83d2d66ed43730a731685",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/checkpolicy-2.9-1.el8.x86_64.rpm"],
)

rpm(
    name = "chkconfig-0__1.19.1-1.el8.x86_64",
    sha256 = "561b5fdadd60370b5d0a91b7ed35df95d7f60650cbade8c7e744323982ac82db",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/chkconfig-1.19.1-1.el8.x86_64.rpm"],
)

rpm(
    name = "coreutils-single-0__8.30-15.el8.x86_64",
    sha256 = "96abd7ec6c1fdfbf845fe71892c50c4ee20dfede79c8070805a0e469c700e2fb",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/coreutils-single-8.30-15.el8.x86_64.rpm"],
)

rpm(
    name = "cpio-0__2.12-11.el8.x86_64",
    sha256 = "e16977e134123c69edc860829d45a5c751ad4befb5576a4a6812b31d6a1ba273",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/cpio-2.12-11.el8.x86_64.rpm"],
)

rpm(
    name = "cracklib-0__2.9.6-15.el8.x86_64",
    sha256 = "dbbc9e20caabc30070354d91f61f383081f6d658e09d3c09e6df8764559e5aca",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/cracklib-2.9.6-15.el8.x86_64.rpm"],
)

rpm(
    name = "cracklib-dicts-0__2.9.6-15.el8.x86_64",
    sha256 = "f1ce23ee43c747a35367dada19ca200a7758c50955ccc44aa946b86b647077ca",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/cracklib-dicts-2.9.6-15.el8.x86_64.rpm"],
)

rpm(
    name = "crypto-policies-0__20221215-1.gitece0092.el8.x86_64",
    sha256 = "29d99b0985833aea0b2590036dcbb03e225877c30a18c707f2e149eaf5ba3697",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/crypto-policies-20221215-1.gitece0092.el8.noarch.rpm"],
)

rpm(
    name = "crypto-policies-scripts-0__20221215-1.gitece0092.el8.x86_64",
    sha256 = "3ac08f29a4b02fc24b115487e033472af427a4f1e315e89eada474cfa6543922",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/crypto-policies-scripts-20221215-1.gitece0092.el8.noarch.rpm"],
)

rpm(
    name = "cryptsetup-libs-0__2.3.7-5.el8.x86_64",
    sha256 = "fe2e1ef00d792f44b27afc53dff8a99405de7496756ae3f5f10e91ba2bd1e460",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/cryptsetup-libs-2.3.7-5.el8.x86_64.rpm"],
)

rpm(
    name = "curl-0__7.61.1-28.el8.x86_64",
    sha256 = "f63798af9d91182a882991fd9ec6780d51c5bd87bb72484f4df61f6d51631732",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/curl-7.61.1-28.el8.x86_64.rpm"],
)

rpm(
    name = "cyrus-sasl-0__2.1.27-6.el8_5.x86_64",
    sha256 = "65a62affe9c99e597aabf117b8439a363761686c496723bc492dbfdcb6f60692",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/cyrus-sasl-2.1.27-6.el8_5.x86_64.rpm"],
)

rpm(
    name = "cyrus-sasl-gssapi-0__2.1.27-6.el8_5.x86_64",
    sha256 = "6c9a8d9adc93d1be7db41fe7327c4dcce144cefad3008e580f5e9cadb6155eb4",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/cyrus-sasl-gssapi-2.1.27-6.el8_5.x86_64.rpm"],
)

rpm(
    name = "cyrus-sasl-lib-0__2.1.27-6.el8_5.x86_64",
    sha256 = "5bd6e1201d8b10c6f01f500c43f63204f1d2ec8a4d8ce53c741e611c81ffb404",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/cyrus-sasl-lib-2.1.27-6.el8_5.x86_64.rpm"],
)

rpm(
    name = "daxctl-libs-0__71.1-4.el8.x86_64",
    sha256 = "332af3c063fdb03d95632dc5010712c4e9ca7416f3049c901558c5aa0c6e445b",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/daxctl-libs-71.1-4.el8.x86_64.rpm"],
)

rpm(
    name = "dbus-1__1.12.8-24.el8.x86_64",
    sha256 = "feba20c1a54cd905cba7ad79665814b084b71fd391f88458d36cc99a0e4786b9",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/dbus-1.12.8-24.el8.x86_64.rpm"],
)

rpm(
    name = "dbus-common-1__1.12.8-24.el8.x86_64",
    sha256 = "5fb132e3a6b3fcedbb13de4ef5004d8c1ee4722cd42f17712e69fbdc1ae70572",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/dbus-common-1.12.8-24.el8.noarch.rpm"],
)

rpm(
    name = "dbus-daemon-1__1.12.8-24.el8.x86_64",
    sha256 = "6b5611899424c5382d9917d74148473535e0e7b9dc7ef8dd74e410b28b5d9342",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/dbus-daemon-1.12.8-24.el8.x86_64.rpm"],
)

rpm(
    name = "dbus-libs-1__1.12.8-24.el8.x86_64",
    sha256 = "4687b9ae45e0bb542c76694db9473c21e88961abc47237156cd9147eaf524be7",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/dbus-libs-1.12.8-24.el8.x86_64.rpm"],
)

rpm(
    name = "dbus-tools-1__1.12.8-24.el8.x86_64",
    sha256 = "a35c85304f8c360779b7488dcc687a95f24a71327de6f33db758f418e0b491b6",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/dbus-tools-1.12.8-24.el8.x86_64.rpm"],
)

rpm(
    name = "device-mapper-8__1.02.181-9.el8.x86_64",
    sha256 = "28f2e3e2a0888e59d23525473d21e3486aabdbbd27b86d40c57b22bbd5a3a323",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/device-mapper-1.02.181-9.el8.x86_64.rpm"],
)

rpm(
    name = "device-mapper-event-8__1.02.181-9.el8.x86_64",
    sha256 = "ebd0610b792ef94ad1740e00b1c5c7678fde6e7cd61d035b1bfc8ed05ae5a6ea",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/device-mapper-event-1.02.181-9.el8.x86_64.rpm"],
)

rpm(
    name = "device-mapper-event-libs-8__1.02.181-9.el8.x86_64",
    sha256 = "fd740286527b20fa3647645882e531904de9665f59adaa815691e32095f491f2",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/device-mapper-event-libs-1.02.181-9.el8.x86_64.rpm"],
)

rpm(
    name = "device-mapper-libs-8__1.02.181-9.el8.x86_64",
    sha256 = "8fd6ecaa19fc86b236fb00d1a816eca2ab84e6531ca6fe318bfc1297caee8e88",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/device-mapper-libs-1.02.181-9.el8.x86_64.rpm"],
)

rpm(
    name = "device-mapper-multipath-libs-0__0.8.4-34.el8.x86_64",
    sha256 = "f4b4bb1ed8a724f4468b7640aae81dd474fb276d0d4fb69877216f2d3ede4761",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/device-mapper-multipath-libs-0.8.4-34.el8.x86_64.rpm"],
)

rpm(
    name = "device-mapper-persistent-data-0__0.9.0-7.el8.x86_64",
    sha256 = "609c2bf12ce2994a0753177e334cde294a96750903c24d8583e7a0674c80485e",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/device-mapper-persistent-data-0.9.0-7.el8.x86_64.rpm"],
)

rpm(
    name = "diffutils-0__3.6-6.el8.x86_64",
    sha256 = "c515d78c64a93d8b469593bff5800eccd50f24b16697ab13bdce81238c38eb77",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/diffutils-3.6-6.el8.x86_64.rpm"],
)

rpm(
    name = "dmidecode-1__3.3-4.el8.x86_64",
    sha256 = "c1347fe2d5621a249ea230e9e8ff2774e538031070a225245154a75428ec67a5",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/dmidecode-3.3-4.el8.x86_64.rpm"],
)

rpm(
    name = "dnsmasq-0__2.79-24.el8.x86_64",
    sha256 = "d93901da01f46867a60b9f5f2bdcfdffd2e896e148b2703c287e369349c6b492",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/dnsmasq-2.79-24.el8.x86_64.rpm"],
)

rpm(
    name = "dracut-0__049-223.git20230119.el8.x86_64",
    sha256 = "46d52da69fffd8922e49bbe37eac472368c85e81d5ef40f3cd5659d57488c281",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/dracut-049-223.git20230119.el8.x86_64.rpm"],
)

rpm(
    name = "e2fsprogs-libs-0__1.45.6-5.el8.x86_64",
    sha256 = "035c5ed68339e632907c3f952098cdc9181ab9138239473903000e6a50446d98",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/e2fsprogs-libs-1.45.6-5.el8.x86_64.rpm"],
)

rpm(
    name = "edk2-ovmf-0__20220126gitbb1bba3d77-3.el8.x86_64",
    sha256 = "e176e6ee0e4e14807e8bc518ee1e41db4788dd355e61102f0d47b9c8b61d9b5a",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/edk2-ovmf-20220126gitbb1bba3d77-3.el8.noarch.rpm"],
)

rpm(
    name = "elfutils-default-yama-scope-0__0.188-3.el8.x86_64",
    sha256 = "fa1c01e489744a0bc3127d7996b9a2527347ace9c97c04d146c2331fd0acb926",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/elfutils-default-yama-scope-0.188-3.el8.noarch.rpm"],
)

rpm(
    name = "elfutils-libelf-0__0.188-3.el8.x86_64",
    sha256 = "746cb30b5c69ddfe1c525b165a036866fbbb38091d9e1565c9b1a3c4fa48f74c",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/elfutils-libelf-0.188-3.el8.x86_64.rpm"],
)

rpm(
    name = "elfutils-libs-0__0.188-3.el8.x86_64",
    sha256 = "1ece046828213af0a1fadaf44cfc456441e3ce2440a1ee32ed22a640b7f87510",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/elfutils-libs-0.188-3.el8.x86_64.rpm"],
)

rpm(
    name = "expat-0__2.2.5-11.el8.x86_64",
    sha256 = "5deba05aa6366108abb5cc764584eec5594f77c052ef02927f0ce0b3b5cc4065",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/expat-2.2.5-11.el8.x86_64.rpm"],
)

rpm(
    name = "file-0__5.33-21.el8.x86_64",
    sha256 = "202e8164df8a6110d58692fa25eaf1d1078a988372943ae73536333237dc3818",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/file-5.33-21.el8.x86_64.rpm"],
)

rpm(
    name = "file-libs-0__5.33-21.el8.x86_64",
    sha256 = "9a51006d0e557e456eb9fc03ff7ed236633d32823dbd46984aca96f379e09f21",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/file-libs-5.33-21.el8.x86_64.rpm"],
)

rpm(
    name = "filesystem-0__3.8-6.el8.x86_64",
    sha256 = "50bdb81d578914e0e88fe6b13550b4c30aac4d72f064fdcd78523df7dd2f64da",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/filesystem-3.8-6.el8.x86_64.rpm"],
)

rpm(
    name = "findutils-1__4.6.0-20.el8.x86_64",
    sha256 = "811eb112646b7d87773c65af47efdca975468f3e5df44aa9944e30de24d83890",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/findutils-4.6.0-20.el8.x86_64.rpm"],
)

rpm(
    name = "fontconfig-0__2.13.1-4.el8.x86_64",
    sha256 = "1d2c61493d72419e85272e8cbc1a0bf3232c81b9bed4707d68f2bbeef2391a55",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/fontconfig-2.13.1-4.el8.x86_64.rpm"],
)

rpm(
    name = "fontpackages-filesystem-0__1.44-22.el8.x86_64",
    sha256 = "700b9050aa490b5eca6d1f8630cbebceb122fce11c370689b5ccb37f5a43ee2f",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/fontpackages-filesystem-1.44-22.el8.noarch.rpm"],
)

rpm(
    name = "freetype-0__2.9.1-9.el8.x86_64",
    sha256 = "0097dc947c987310bb5cbcb9976594eac1e1d111e065ffee150abc2d69b8d709",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/freetype-2.9.1-9.el8.x86_64.rpm"],
)

rpm(
    name = "fribidi-0__1.0.4-9.el8.x86_64",
    sha256 = "6540f56f1d5f191d91e8445d7156bf7fae954c18f07c7191bd0cb0ef38455e05",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/fribidi-1.0.4-9.el8.x86_64.rpm"],
)

rpm(
    name = "fuse-0__2.9.7-16.el8.x86_64",
    sha256 = "c208aa2f2f216a2172b1d9fa82bcad1b201e62f9a3101f4d52fb3de54ed28596",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/fuse-2.9.7-16.el8.x86_64.rpm"],
)

rpm(
    name = "fuse-common-0__3.3.0-16.el8.x86_64",
    sha256 = "d637dfd117080f52f1a60444b6c09aaf65a535844cacce05945d1d691b8d7043",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/fuse-common-3.3.0-16.el8.x86_64.rpm"],
)

rpm(
    name = "fuse-libs-0__2.9.7-16.el8.x86_64",
    sha256 = "77fff0f92a55307b7df2334bc9cc2998c024586abd96286a251919b0509f0473",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/fuse-libs-2.9.7-16.el8.x86_64.rpm"],
)

rpm(
    name = "gawk-0__4.2.1-4.el8.x86_64",
    sha256 = "ff4438c2dff5bf933d7874fd55f131ca6ee067f8fb4324c89719d63e60b40aba",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/gawk-4.2.1-4.el8.x86_64.rpm"],
)

rpm(
    name = "gdbm-1__1.18-2.el8.x86_64",
    sha256 = "fa1751b26519b9637cf3f0a25ea1874eb2df005dde1e1371a3f13d0c9a38b9ca",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/gdbm-1.18-2.el8.x86_64.rpm"],
)

rpm(
    name = "gdbm-libs-1__1.18-2.el8.x86_64",
    sha256 = "eddcea96342c8cfaa60b79fc2c66cb8c5b0038c3b11855abe55e659b2cad6199",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/gdbm-libs-1.18-2.el8.x86_64.rpm"],
)

rpm(
    name = "gettext-0__0.19.8.1-17.el8.x86_64",
    sha256 = "829c842bbd79dca18d37198414626894c44e5b8faf0cce0054ca0ba6623ae136",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/gettext-0.19.8.1-17.el8.x86_64.rpm"],
)

rpm(
    name = "gettext-libs-0__0.19.8.1-17.el8.x86_64",
    sha256 = "ade52756aaf236e77dadd6cf97716821141c2759129ca7808524ab79607bb4c4",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/gettext-libs-0.19.8.1-17.el8.x86_64.rpm"],
)

rpm(
    name = "glib-networking-0__2.56.1-1.1.el8.x86_64",
    sha256 = "a7f9ae54f45ca4fcecf78d9885d12a789f7325119794178bfa2814c6185a953d",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/glib-networking-2.56.1-1.1.el8.x86_64.rpm"],
)

rpm(
    name = "glib2-0__2.56.4-161.el8.x86_64",
    sha256 = "d719ce836f972f57e577f315267f6b5177cc8f8cc9687a8432f1e22cf575bb81",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/glib2-2.56.4-161.el8.x86_64.rpm"],
)

rpm(
    name = "glibc-0__2.28-224.el8.x86_64",
    sha256 = "d435b2974794c7acd6b263676ae5f80fdf9ffaee30ba92f86eb0f0dbc07740db",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/glibc-2.28-224.el8.x86_64.rpm"],
)

rpm(
    name = "glibc-common-0__2.28-224.el8.x86_64",
    sha256 = "21c3069e5de0ffa8800b2e03112079582025c121a6da2fe66c63595db1f4e63b",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/glibc-common-2.28-224.el8.x86_64.rpm"],
)

rpm(
    name = "glibc-langpack-en-0__2.28-224.el8.x86_64",
    sha256 = "c6e43b3488320903ef9ef1b9acad0be3e323ae6929f8937205335a1310ecd60f",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/glibc-langpack-en-2.28-224.el8.x86_64.rpm"],
)

rpm(
    name = "glusterfs-0__6.0-56.4.el8.x86_64",
    sha256 = "83b47312daf82365b52b67523fb24fbe2cd48ff344e6a07df2845a920c309444",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/glusterfs-6.0-56.4.el8.x86_64.rpm"],
)

rpm(
    name = "glusterfs-api-0__6.0-56.4.el8.x86_64",
    sha256 = "26926dfc4dc3fc8341cdf38fad0a4d23426c0b60521a7ef6f1a4142f8b9272dd",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/glusterfs-api-6.0-56.4.el8.x86_64.rpm"],
)

rpm(
    name = "glusterfs-cli-0__6.0-56.4.el8.x86_64",
    sha256 = "32a37a1e248acb2441f6b72996e6614b72984270373f1339f3c1d5bcbee29185",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/glusterfs-cli-6.0-56.4.el8.x86_64.rpm"],
)

rpm(
    name = "glusterfs-client-xlators-0__6.0-56.4.el8.x86_64",
    sha256 = "4e74285c078ca8b75ba3d995ac78f4eb8be69a743a333804550fd4de27dddf66",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/glusterfs-client-xlators-6.0-56.4.el8.x86_64.rpm"],
)

rpm(
    name = "glusterfs-libs-0__6.0-56.4.el8.x86_64",
    sha256 = "82613d82932889856e109d734220e059adf67da0e946b77896f19d5d19f5bd16",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/glusterfs-libs-6.0-56.4.el8.x86_64.rpm"],
)

rpm(
    name = "gmp-1__6.1.2-10.el8.x86_64",
    sha256 = "3b96e2c7d5cd4b49bfde8e52c8af6ff595c91438e50856e468f14a049d8511e2",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/gmp-6.1.2-10.el8.x86_64.rpm"],
)

rpm(
    name = "gnupg2-0__2.2.20-3.el8.x86_64",
    sha256 = "8c44c980dd9a6a42ccb93578d7e6e1940d36d2da0a5a99d783189c43b2ad6d5f",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/gnupg2-2.2.20-3.el8.x86_64.rpm"],
)

rpm(
    name = "gnutls-0__3.6.16-6.el8.x86_64",
    sha256 = "db83285511f8799526cf894bbd481bbc44c4c60dbdd61d3bfd2c96324190c95b",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/gnutls-3.6.16-6.el8.x86_64.rpm"],
)

rpm(
    name = "gnutls-dane-0__3.6.16-6.el8.x86_64",
    sha256 = "b9a63d958c807b4eb40ee01c4679727158872c31f9d359974c5bb43488b29476",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/gnutls-dane-3.6.16-6.el8.x86_64.rpm"],
)

rpm(
    name = "gnutls-utils-0__3.6.16-6.el8.x86_64",
    sha256 = "ce260c04822b7b38fe009180c1ada312f81ff1b58a35d0e2734de02e1d7bb8ca",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/gnutls-utils-3.6.16-6.el8.x86_64.rpm"],
)

rpm(
    name = "graphite2-0__1.3.10-10.el8.x86_64",
    sha256 = "0f9c3ee5f54ed296f99219bd70fa4f869c4c9986e3766d813a76a0cc5ecee24e",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/graphite2-1.3.10-10.el8.x86_64.rpm"],
)

rpm(
    name = "grep-0__3.1-6.el8.x86_64",
    sha256 = "3f8ffe48bb481a5db7cbe42bf73b839d872351811e5df41b2f6697c61a030487",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/grep-3.1-6.el8.x86_64.rpm"],
)

rpm(
    name = "groff-base-0__1.22.3-18.el8.x86_64",
    sha256 = "b00855013100d3796e9ed6d82b1ab2d4dc7f4a3a3fa2e186f6de8523577974a0",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/groff-base-1.22.3-18.el8.x86_64.rpm"],
)

rpm(
    name = "gsettings-desktop-schemas-0__3.32.0-6.el8.x86_64",
    sha256 = "4f05013bb8d2d2173d83dc667cafe942bdd0299fb21cb6bebe0f306d92df1842",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/gsettings-desktop-schemas-3.32.0-6.el8.x86_64.rpm"],
)

rpm(
    name = "gssproxy-0__0.8.0-21.el8.x86_64",
    sha256 = "05325a046fdc9ef34248053ae08cee10ed3422f481911dd21bad59fae3ddd22d",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/gssproxy-0.8.0-21.el8.x86_64.rpm"],
)

rpm(
    name = "gstreamer1-0__1.16.1-2.el8.x86_64",
    sha256 = "f15ce668cd55f1d5df62902d98ade38a057e3c782549dca3c45ce038b9ae2968",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/gstreamer1-1.16.1-2.el8.x86_64.rpm"],
)

rpm(
    name = "gstreamer1-plugins-base-0__1.16.1-2.el8.x86_64",
    sha256 = "755c97a2a0b3460f51c5e70b18ca207eb3b68c1647d6949666f0dfd739dce319",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/gstreamer1-plugins-base-1.16.1-2.el8.x86_64.rpm"],
)

rpm(
    name = "gzip-0__1.9-13.el8.x86_64",
    sha256 = "1cc189e4991fc6b3526f7eebc9f798b8922e70d60a12ba499b6e0329eb473cea",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/gzip-1.9-13.el8.x86_64.rpm"],
)

rpm(
    name = "harfbuzz-0__1.7.5-3.el8.x86_64",
    sha256 = "49c652f3d967e944b9d0ad9dea63e8942626d3b9f40fde12cfb0d3e924a82053",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/harfbuzz-1.7.5-3.el8.x86_64.rpm"],
)

rpm(
    name = "hexedit-0__1.2.13-12.el8.x86_64",
    sha256 = "4538e44d3ebff3f9323b59171767bca2b7f5244dd90141de101856ad4f4643f5",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/hexedit-1.2.13-12.el8.x86_64.rpm"],
)

rpm(
    name = "hivex-0__1.3.18-23.module_el8.6.0__plus__983__plus__a7505f3f.x86_64",
    sha256 = "d24f86d286bd2294de8b3c2931c3f851495cd12f76a24705425635f55eaf1147",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/hivex-1.3.18-23.module_el8.6.0+983+a7505f3f.x86_64.rpm"],
)

rpm(
    name = "hwdata-0__0.314-8.15.el8.x86_64",
    sha256 = "0b644a133d75c2b912a559cdcfefe20712e61a2554f48d154e6a4be54ac966fd",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/hwdata-0.314-8.15.el8.noarch.rpm"],
)

rpm(
    name = "info-0__6.5-7.el8_5.x86_64",
    sha256 = "63f03261cc8109b2fb61002ca50c93e52acb9cfd8382d139e8de6623394051e8",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/info-6.5-7.el8_5.x86_64.rpm"],
)

rpm(
    name = "iproute-0__5.18.0-1.el8.x86_64",
    sha256 = "7ae4b834f060d111db19fa3cf6f6266d4c6fb56992b0347145799d7ff9f03d3c",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/iproute-5.18.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "iproute-tc-0__5.18.0-1.el8.x86_64",
    sha256 = "bca80255b377f2a715c1fa2023485cd8fd03f2bab2a873faa0e5879082bca1c9",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/iproute-tc-5.18.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "iptables-0__1.8.4-24.el8.x86_64",
    sha256 = "e4d26dec2832a8177e76d0d287a70dfaa57499ebf954610c215e449b9190492e",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/iptables-1.8.4-24.el8.x86_64.rpm"],
)

rpm(
    name = "iptables-ebtables-0__1.8.4-24.el8.x86_64",
    sha256 = "25b801169050e77395d204d9beb30dccc97674ea6efec3759e80bae71fe1c683",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/iptables-ebtables-1.8.4-24.el8.x86_64.rpm"],
)

rpm(
    name = "iptables-libs-0__1.8.4-24.el8.x86_64",
    sha256 = "cf70e436e2fe912f419579500fd30512be5420009d63a82aacc47767b32901d5",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/iptables-libs-1.8.4-24.el8.x86_64.rpm"],
)

rpm(
    name = "ipxe-roms-qemu-0__20181214-11.git133f4c47.el8.x86_64",
    sha256 = "14640176ccf8c67c986132466915d3fa2c049076e7a2633b5d8e79cbb5e03a24",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/ipxe-roms-qemu-20181214-11.git133f4c47.el8.noarch.rpm"],
)

rpm(
    name = "iscsi-initiator-utils-0__6.2.1.4-4.git095f59c.el8.x86_64",
    sha256 = "0a4c90baac48f116789645c8dc1351c6ede1dfa6c5664d09779f51858d2dde1c",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/iscsi-initiator-utils-6.2.1.4-4.git095f59c.el8.x86_64.rpm"],
)

rpm(
    name = "iscsi-initiator-utils-iscsiuio-0__6.2.1.4-4.git095f59c.el8.x86_64",
    sha256 = "7ac58a3b9990da620d568ff0b3cf7e3555f0bad5f2d9c86c771e84d5939e81e5",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/iscsi-initiator-utils-iscsiuio-6.2.1.4-4.git095f59c.el8.x86_64.rpm"],
)

rpm(
    name = "isns-utils-libs-0__0.99-1.el8.x86_64",
    sha256 = "5830a9484eb786849dd73fce6f2b20d5d42e779d687842f60c8b588c962e5e40",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/isns-utils-libs-0.99-1.el8.x86_64.rpm"],
)

rpm(
    name = "iso-codes-0__3.79-2.el8.x86_64",
    sha256 = "f5a0a39b40f2af0b74ec47f6a5e00f7772ac8bd347c793b7deac84d3d8d7d47a",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/iso-codes-3.79-2.el8.noarch.rpm"],
)

rpm(
    name = "jansson-0__2.14-1.el8.x86_64",
    sha256 = "f825b85b4506a740fb2f85b9a577c51264f3cfe792dd8b2bf8963059cc77c3c4",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/jansson-2.14-1.el8.x86_64.rpm"],
)

rpm(
    name = "json-c-0__0.13.1-3.el8.x86_64",
    sha256 = "5035057553b61cb389c67aa2c29d99c8e0c1677369dad179d683942ccee90b3f",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/json-c-0.13.1-3.el8.x86_64.rpm"],
)

rpm(
    name = "json-glib-0__1.4.4-1.el8.x86_64",
    sha256 = "98a6386df94fc9595365c3ecbc630708420fa68d1774614a723dec4a55e84b9c",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/json-glib-1.4.4-1.el8.x86_64.rpm"],
)

rpm(
    name = "kernel-core-0__4.18.0-448.el8.x86_64",
    sha256 = "ae367028decb901507137eb8d056fbd96348fd32226eb2d1128ce716bfb6e761",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/kernel-core-4.18.0-448.el8.x86_64.rpm"],
)

rpm(
    name = "keyutils-0__1.5.10-9.el8.x86_64",
    sha256 = "4b6adc20f41b59b787291588a3de9182404199db575067282965878f693c40cc",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/keyutils-1.5.10-9.el8.x86_64.rpm"],
)

rpm(
    name = "keyutils-libs-0__1.5.10-9.el8.x86_64",
    sha256 = "423329269c719b96ada88a27325e1923e764a70672e0dc6817e22eff07a9af7b",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/keyutils-libs-1.5.10-9.el8.x86_64.rpm"],
)

rpm(
    name = "kmod-0__25-19.el8.x86_64",
    sha256 = "37c299fdaa42efb0d653ba5e22c83bd20833af1244b66ed6ea880e75c1672dd2",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/kmod-25-19.el8.x86_64.rpm"],
)

rpm(
    name = "kmod-libs-0__25-19.el8.x86_64",
    sha256 = "46a2ddc6067ed12089f04f2255c57117992807d707e280fc002f3ce786fc2abf",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/kmod-libs-25-19.el8.x86_64.rpm"],
)

rpm(
    name = "krb5-libs-0__1.18.2-22.el8.x86_64",
    sha256 = "1dc1106dda34b328115dff7b2eca007dd93befb0bfa6a66c619f4b5637f6e004",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/krb5-libs-1.18.2-22.el8.x86_64.rpm"],
)

rpm(
    name = "less-0__530-1.el8.x86_64",
    sha256 = "f94172554b8ceeab97b560d0b05c2e2df4b2e737471adce6eca82fd3209be254",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/less-530-1.el8.x86_64.rpm"],
)

rpm(
    name = "libX11-0__1.6.8-5.el8.x86_64",
    sha256 = "2ab1fef0235ca1cafbe23ad6c9dbe3cd5d48ab99561f7e880456606a1ea78ae4",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libX11-1.6.8-5.el8.x86_64.rpm"],
)

rpm(
    name = "libX11-common-0__1.6.8-5.el8.x86_64",
    sha256 = "53760c2d7e17f31bd1f999cb448e902d4ba68eff0f99f6203d85998cd4c44918",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libX11-common-1.6.8-5.el8.noarch.rpm"],
)

rpm(
    name = "libX11-xcb-0__1.6.8-5.el8.x86_64",
    sha256 = "d8d58813823c960f344bdebf4ed888b53781c81175e97badd814a86dc811b362",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libX11-xcb-1.6.8-5.el8.x86_64.rpm"],
)

rpm(
    name = "libXau-0__1.0.9-3.el8.x86_64",
    sha256 = "49d972c660b9238dd1d58ab5952285b77e440820bf4563cce2b5ecd2f6ceba78",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libXau-1.0.9-3.el8.x86_64.rpm"],
)

rpm(
    name = "libXext-0__1.3.4-1.el8.x86_64",
    sha256 = "9869db60ee2b6d8f2115937857fb0586802720a75e043baf21514011a9fa79aa",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libXext-1.3.4-1.el8.x86_64.rpm"],
)

rpm(
    name = "libXfixes-0__5.0.3-7.el8.x86_64",
    sha256 = "81f7df4c736963636c9ebab7441ca4f4e41a7483ef6e7b2ac0d1bf37afe52a14",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libXfixes-5.0.3-7.el8.x86_64.rpm"],
)

rpm(
    name = "libXft-0__2.3.3-1.el8.x86_64",
    sha256 = "ab754d37388e0ecb52152e41c9560392dd0d504939f850ff25d9794090f8b101",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libXft-2.3.3-1.el8.x86_64.rpm"],
)

rpm(
    name = "libXrender-0__0.9.10-7.el8.x86_64",
    sha256 = "11ac209220f3a53a762adebb4eeb43190e02ef7cdad2c54bcb474b206f7eb6f2",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libXrender-0.9.10-7.el8.x86_64.rpm"],
)

rpm(
    name = "libXv-0__1.0.11-7.el8.x86_64",
    sha256 = "e04aeb7921dc1864379f670172c69d2e6241c0ca602b7bdee42079596910a4c3",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libXv-1.0.11-7.el8.x86_64.rpm"],
)

rpm(
    name = "libXxf86vm-0__1.1.4-9.el8.x86_64",
    sha256 = "5813a48905fafc027e4b71b8113e654f23c963a9526015ec4fd0738b68de264a",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libXxf86vm-1.1.4-9.el8.x86_64.rpm"],
)

rpm(
    name = "libacl-0__2.2.53-1.el8.x86_64",
    sha256 = "4973664648b7ed9278bf29074ec6a60a9f660aa97c23a283750483f64429d5bb",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libacl-2.2.53-1.el8.x86_64.rpm"],
)

rpm(
    name = "libaio-0__0.3.112-1.el8.x86_64",
    sha256 = "2c63399bee449fb6e921671a9bbf3356fda73f890b578820f7d926202e98a479",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libaio-0.3.112-1.el8.x86_64.rpm"],
)

rpm(
    name = "libarchive-0__3.3.3-5.el8.x86_64",
    sha256 = "d2e208573fde1934bd11c52a45edd6c360d365e0c675b43043fe863a248f5f5b",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libarchive-3.3.3-5.el8.x86_64.rpm"],
)

rpm(
    name = "libassuan-0__2.5.1-3.el8.x86_64",
    sha256 = "b49e8c674e462e3f494e825c5fca64002008cbf7a47bf131aa98b7f41678a6eb",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libassuan-2.5.1-3.el8.x86_64.rpm"],
)

rpm(
    name = "libattr-0__2.4.48-3.el8.x86_64",
    sha256 = "a02e1344ccde1747501ceeeff37df4f18149fb79b435aa22add08cff6bab3a5a",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libattr-2.4.48-3.el8.x86_64.rpm"],
)

rpm(
    name = "libbasicobjects-0__0.1.1-40.el8.x86_64",
    sha256 = "cc4c7d14093bc2dbd690aab88523ecc9dc1c90810191452b7e1ac756c99629ba",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libbasicobjects-0.1.1-40.el8.x86_64.rpm"],
)

rpm(
    name = "libblkid-0__2.32.1-39.el8.x86_64",
    sha256 = "8368b8462e9763cdc7a9586ca1a266d3858aafa5ad82c473cb4e1c0bf2d6c755",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libblkid-2.32.1-39.el8.x86_64.rpm"],
)

rpm(
    name = "libbpf-0__0.5.0-1.el8.x86_64",
    sha256 = "4d25308c27041d8a88a3340be12591e9bd46c9aebbe4195ee5d2f712d63ce033",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libbpf-0.5.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "libcap-0__2.48-4.el8.x86_64",
    sha256 = "34f69bed9ae0f5ba314a62172e8cfd9cf6795cb0c3bd29f15d174fc2a0acbb5b",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libcap-2.48-4.el8.x86_64.rpm"],
)

rpm(
    name = "libcap-ng-0__0.7.11-1.el8.x86_64",
    sha256 = "15c3c696ec2e21f48e951f426d3c77b53b579605b8dd89843b35c9ab9b1d7e69",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libcap-ng-0.7.11-1.el8.x86_64.rpm"],
)

rpm(
    name = "libcollection-0__0.7.0-40.el8.x86_64",
    sha256 = "9282a11d3792e771dfef1df80c6234fa3845638e9a27b438362810f8dfc5d208",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libcollection-0.7.0-40.el8.x86_64.rpm"],
)

rpm(
    name = "libcom_err-0__1.45.6-5.el8.x86_64",
    sha256 = "4e4f13acac0477f0a121812107a9939ea2164eebab052813f1618d5b7df5d87a",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libcom_err-1.45.6-5.el8.x86_64.rpm"],
)

rpm(
    name = "libconfig-0__1.5-9.el8.x86_64",
    sha256 = "a4a2c7c0e2f454abae61dddbf4286a0b3617a8159fd20659bddbcedd8eaaa80c",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libconfig-1.5-9.el8.x86_64.rpm"],
)

rpm(
    name = "libcroco-0__0.6.12-4.el8_2.1.x86_64",
    sha256 = "87f2a4d80cf4f6a958f3662c6a382edefc32a5ad2c364a7f3c40337cf2b1e8ba",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libcroco-0.6.12-4.el8_2.1.x86_64.rpm"],
)

rpm(
    name = "libcurl-minimal-0__7.61.1-28.el8.x86_64",
    sha256 = "dbffb9b9dc6814f21aa007a0797cad5fb04dc421fe528d6330e8f14043a62bae",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libcurl-minimal-7.61.1-28.el8.x86_64.rpm"],
)

rpm(
    name = "libdatrie-0__0.2.9-7.el8.x86_64",
    sha256 = "7d43fda5ced8faf64d09cb3c47dcb6c9aa1fd936fc49f8609af29780c7a75f90",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libdatrie-0.2.9-7.el8.x86_64.rpm"],
)

rpm(
    name = "libdb-0__5.3.28-42.el8_4.x86_64",
    sha256 = "058f77432592f4337039cbb7a4e5f680020d8b85a477080c01d96a7728de6934",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libdb-5.3.28-42.el8_4.x86_64.rpm"],
)

rpm(
    name = "libdb-utils-0__5.3.28-42.el8_4.x86_64",
    sha256 = "ceb3dbd9e0d39d3e6b566eaf05359de4dd9a18d09da9238f2319f66f7cfebf7b",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libdb-utils-5.3.28-42.el8_4.x86_64.rpm"],
)

rpm(
    name = "libdrm-0__2.4.114-1.el8.x86_64",
    sha256 = "af65274314c0e0423fd6430d19f79a0f11ec3f3f23fba1c10ea7ebdf47443cc9",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libdrm-2.4.114-1.el8.x86_64.rpm"],
)

rpm(
    name = "libepoxy-0__1.5.8-1.el8.x86_64",
    sha256 = "a825b6169fbd3377aed37ee63114aff24ac1a0ae123710c4559a56564fb0c15a",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libepoxy-1.5.8-1.el8.x86_64.rpm"],
)

rpm(
    name = "libev-0__4.24-6.el8.x86_64",
    sha256 = "83549217540abd259f74b84d9359ab200c2cbe6e9b2e25a73d7236cf441aed4c",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libev-4.24-6.el8.x86_64.rpm"],
)

rpm(
    name = "libevent-0__2.1.8-5.el8.x86_64",
    sha256 = "746bac6bb011a586d42bd82b2f8b25bac72c9e4bbd4c19a34cf88eadb1d83873",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libevent-2.1.8-5.el8.x86_64.rpm"],
)

rpm(
    name = "libfdisk-0__2.32.1-39.el8.x86_64",
    sha256 = "7e8c4ea7f5ea7339bba2db65fa8737e7acba431aa9a50b11cb741fa61aaf374d",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libfdisk-2.32.1-39.el8.x86_64.rpm"],
)

rpm(
    name = "libfdt-0__1.6.0-1.el8.x86_64",
    sha256 = "1788b4786715c45a1ac90ca9f413ef51f2cdd03170a981e0ef13eab204f44429",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libfdt-1.6.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "libffi-0__3.1-24.el8.x86_64",
    sha256 = "3a0b75d820053e5a75f3a9a04fa2b78a7ac559140d7ce98f0d684cd8433ece81",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libffi-3.1-24.el8.x86_64.rpm"],
)

rpm(
    name = "libgcc-0__8.5.0-18.el8.x86_64",
    sha256 = "c365e8777d26c19cd3ceac4023cda40edbef2a1f9022da1fb509646875631b20",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libgcc-8.5.0-18.el8.x86_64.rpm"],
)

rpm(
    name = "libgcrypt-0__1.8.5-7.el8.x86_64",
    sha256 = "01541f1263532f80114111a44f797d6a8eed75744db997e85fddd021e636c5bb",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libgcrypt-1.8.5-7.el8.x86_64.rpm"],
)

rpm(
    name = "libglvnd-1__1.3.4-1.el8.x86_64",
    sha256 = "a94d8debdf9e1f20dc561baaa1c5903ef85c511f2b647092b5d8908ccfbf6a6a",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libglvnd-1.3.4-1.el8.x86_64.rpm"],
)

rpm(
    name = "libglvnd-egl-1__1.3.4-1.el8.x86_64",
    sha256 = "0c7e300aae2f33e48ae5bedbbcf9c6b50af18477d9493075c73355c7fe080b43",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libglvnd-egl-1.3.4-1.el8.x86_64.rpm"],
)

rpm(
    name = "libglvnd-gles-1__1.3.4-1.el8.x86_64",
    sha256 = "77f73a543253876ab922320e48b6025b019fa0a109a43da7c1bffe7f0a096522",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libglvnd-gles-1.3.4-1.el8.x86_64.rpm"],
)

rpm(
    name = "libglvnd-glx-1__1.3.4-1.el8.x86_64",
    sha256 = "bf40ab7dbe4ae55fb5403204df6b9b27013898cdb450da39e8e19a2c4229aea5",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libglvnd-glx-1.3.4-1.el8.x86_64.rpm"],
)

rpm(
    name = "libgomp-0__8.5.0-18.el8.x86_64",
    sha256 = "40083ff0ecce7644d1a84d5bd8ba3109321b25bdf5b8a46bb1da85aa9c23d421",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libgomp-8.5.0-18.el8.x86_64.rpm"],
)

rpm(
    name = "libgpg-error-0__1.31-1.el8.x86_64",
    sha256 = "845a0732d9d7a01b909124cd8293204764235c2d856227c7a74dfa0e38113e34",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libgpg-error-1.31-1.el8.x86_64.rpm"],
)

rpm(
    name = "libguestfs-1__1.44.0-9.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "6845108219bcec3306aa45bbec47a0d8d03b867d485b3eb15bb81d7c3bdb728a",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libguestfs-1.44.0-9.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libguestfs-tools-c-1__1.44.0-9.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "2232f7e2f300fce95e29ab761e35b0c67458841a1b5c41a1b544e3f9f2721621",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libguestfs-tools-c-1.44.0-9.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libguestfs-winsupport-0__8.6-1.module_el8.6.0__plus__983__plus__a7505f3f.x86_64",
    sha256 = "9e36cd50c86ccd4486c8e949a5d990e3f5ed727f0138879d69f76cac9480e083",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libguestfs-winsupport-8.6-1.module_el8.6.0+983+a7505f3f.x86_64.rpm"],
)

rpm(
    name = "libibverbs-0__41.0-1.el8.x86_64",
    sha256 = "888b1ce059dfaf1b8277cac3529970114ba1cadc75fbcf9410f3031451ab7e30",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libibverbs-41.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "libicu-0__60.3-2.el8_1.x86_64",
    sha256 = "d703112d21afadf069e0ba6ef2a34b0ef760ccc969a2b7dd5d38761113c3d17e",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libicu-60.3-2.el8_1.x86_64.rpm"],
)

rpm(
    name = "libidn2-0__2.2.0-1.el8.x86_64",
    sha256 = "7e08785bd3cc0e09f9ab4bf600b98b705203d552cbb655269a939087987f1694",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libidn2-2.2.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "libini_config-0__1.3.1-40.el8.x86_64",
    sha256 = "4822758f341f9cac045d5d55c57b2a6ae88d3fcfa2e882900af9dc11d5154427",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libini_config-1.3.1-40.el8.x86_64.rpm"],
)

rpm(
    name = "libiscsi-0__1.18.0-8.module_el8.6.0__plus__983__plus__a7505f3f.x86_64",
    sha256 = "77cd7d2f930f737ced7b548e23a37b21ef5bbd7ebc07e147a815b9b6ad76957e",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libiscsi-1.18.0-8.module_el8.6.0+983+a7505f3f.x86_64.rpm"],
)

rpm(
    name = "libjpeg-turbo-0__1.5.3-12.el8.x86_64",
    sha256 = "94b6e9d7ebd6d3bee36ac8b5c381a305bc16158a7de5bf7b71ddf2f41f10f03c",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libjpeg-turbo-1.5.3-12.el8.x86_64.rpm"],
)

rpm(
    name = "libkcapi-0__1.2.0-2.el8.x86_64",
    sha256 = "42f48b1707318215f904134e014d00fac2d811ccc01943abc718b31ef05c0f34",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libkcapi-1.2.0-2.el8.x86_64.rpm"],
)

rpm(
    name = "libkcapi-hmaccalc-0__1.2.0-2.el8.x86_64",
    sha256 = "80ffd3c1ca47e469c9d69b9e88d5b385ba081e55412238ced56fecd996afdf8e",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libkcapi-hmaccalc-1.2.0-2.el8.x86_64.rpm"],
)

rpm(
    name = "libksba-0__1.3.5-8.el8.x86_64",
    sha256 = "8054ca806450e99f1a65d52315229d036cb495ffddfd3f9fccb44e05d0108b46",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libksba-1.3.5-8.el8.x86_64.rpm"],
)

rpm(
    name = "libmnl-0__1.0.4-6.el8.x86_64",
    sha256 = "30fab73ee155f03dbbd99c1e30fe59dfba4ae8fdb2e7213451ccc36d6918bfcc",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libmnl-1.0.4-6.el8.x86_64.rpm"],
)

rpm(
    name = "libmodman-0__2.0.1-17.el8.x86_64",
    sha256 = "c3b8c553b166491d3114793e198cd1aad95e494d177af8d0dc7180b8b841124d",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libmodman-2.0.1-17.el8.x86_64.rpm"],
)

rpm(
    name = "libmount-0__2.32.1-39.el8.x86_64",
    sha256 = "199c6968b0caa6fbe7f413c704e8a22f9915c54883809fd4f61c6327e2eb45c0",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libmount-2.32.1-39.el8.x86_64.rpm"],
)

rpm(
    name = "libnetfilter_conntrack-0__1.0.6-5.el8.x86_64",
    sha256 = "224100af3ecfc80c416796ec02c7c4dd113a38d42349d763485f3b42f260493f",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libnetfilter_conntrack-1.0.6-5.el8.x86_64.rpm"],
)

rpm(
    name = "libnfnetlink-0__1.0.1-13.el8.x86_64",
    sha256 = "cec98aa5fbefcb99715921b493b4f92d34c4eeb823e9c8741aa75e280def89f1",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libnfnetlink-1.0.1-13.el8.x86_64.rpm"],
)

rpm(
    name = "libnfsidmap-1__2.3.3-59.el8.x86_64",
    sha256 = "5caa14840721f668431d5e6b217d1b2b166b67e7ef742dcbae8041c4b32e33e5",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libnfsidmap-2.3.3-59.el8.x86_64.rpm"],
)

rpm(
    name = "libnftnl-0__1.1.5-5.el8.x86_64",
    sha256 = "293e1f0f44a9c1d5dedbe831dff3049fad9e88c5f0e281d889f427603ac51fa6",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libnftnl-1.1.5-5.el8.x86_64.rpm"],
)

rpm(
    name = "libnghttp2-0__1.33.0-3.el8_2.1.x86_64",
    sha256 = "0126a384853d46484dec98601a4cb4ce58b2e0411f8f7ef09937174dd5975bac",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libnghttp2-1.33.0-3.el8_2.1.x86_64.rpm"],
)

rpm(
    name = "libnl3-0__3.7.0-1.el8.x86_64",
    sha256 = "9ce7aa4d7bd810448d9fb3aa85a66cca00950f7c2c59bc9721ced3e4f3ad2885",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libnl3-3.7.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "libnsl2-0__1.2.0-2.20180605git4a062cf.el8.x86_64",
    sha256 = "5846c73edfa2ff673989728e9621cce6a1369eb2f8a269ac5205c381a10d327a",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libnsl2-1.2.0-2.20180605git4a062cf.el8.x86_64.rpm"],
)

rpm(
    name = "libogg-2__1.3.2-10.el8.x86_64",
    sha256 = "35f80ecc7540818e702e49c13cce081bda78ac28087247acf71e6d774e6f0c3e",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libogg-1.3.2-10.el8.x86_64.rpm"],
)

rpm(
    name = "libosinfo-0__1.9.0-3.el8.x86_64",
    sha256 = "671f99ff52154d3c1d4f23a92099f82c131e48bb931fd78c7ee7a6174eb760f8",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libosinfo-1.9.0-3.el8.x86_64.rpm"],
)

rpm(
    name = "libpath_utils-0__0.2.1-40.el8.x86_64",
    sha256 = "df004d035a915323c8cdc36d2a59ddfbc9666712a2afcb90b1d0418a5cd779d1",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libpath_utils-0.2.1-40.el8.x86_64.rpm"],
)

rpm(
    name = "libpcap-14__1.9.1-5.el8.x86_64",
    sha256 = "7f429477c26b4650a3eca4a27b3972ff0857c843bdb4d8fcb02086da111ce5fd",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libpcap-1.9.1-5.el8.x86_64.rpm"],
)

rpm(
    name = "libpciaccess-0__0.14-1.el8.x86_64",
    sha256 = "759386be8f49257266ac614432b762b8e486a89aac5d5f7a581a0330efb59c77",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libpciaccess-0.14-1.el8.x86_64.rpm"],
)

rpm(
    name = "libpipeline-0__1.5.0-2.el8.x86_64",
    sha256 = "9eb9c1a67c5be04487cc133bdb8498eaf260e4d930a0143d2e1aa772e3d6cf64",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libpipeline-1.5.0-2.el8.x86_64.rpm"],
)

rpm(
    name = "libpmem-0__1.12.1-1.module_el8.8.0__plus__1231__plus__994ef5f7.x86_64",
    sha256 = "631f555b4816b73e9f0c5cbf76136d587a93ca03ba735747ac03fc6c6a73bad2",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libpmem-1.12.1-1.module_el8.8.0+1231+994ef5f7.x86_64.rpm"],
)

rpm(
    name = "libpng-2__1.6.34-5.el8.x86_64",
    sha256 = "cc2f054cf7ef006faf0b179701838ff8632c3ac5f45a0199a13f9c237f632b82",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libpng-1.6.34-5.el8.x86_64.rpm"],
)

rpm(
    name = "libproxy-0__0.4.15-5.2.el8.x86_64",
    sha256 = "c9597eecf39a25497b2ac3c69bc9777eda05b9eaa6d5d29d004a81d71a45d0d7",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libproxy-0.4.15-5.2.el8.x86_64.rpm"],
)

rpm(
    name = "libpwquality-0__1.4.4-5.el8.x86_64",
    sha256 = "4a7159ebfb7914f23f009981a38fcbec8368b243b20dfed6326a6dade95cf3a2",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libpwquality-1.4.4-5.el8.x86_64.rpm"],
)

rpm(
    name = "librados2-1__12.2.7-9.el8.x86_64",
    sha256 = "26fc737517bc0b60150e662337000007299d7579376370bc9b907a7fe446a3f0",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/librados2-12.2.7-9.el8.x86_64.rpm"],
)

rpm(
    name = "librbd1-1__12.2.7-9.el8.x86_64",
    sha256 = "f149e46f0f6a31f1af8bdc52385098c66c4c9fa538b5087ed98c357077463128",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/librbd1-12.2.7-9.el8.x86_64.rpm"],
)

rpm(
    name = "librdmacm-0__41.0-1.el8.x86_64",
    sha256 = "caf52cd9c97677b5684730ad61f8abe464cfc41d332b3f4d4887fb2e8ea87916",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/librdmacm-41.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "libref_array-0__0.1.5-40.el8.x86_64",
    sha256 = "fe313a84d495537d5d1fc2aee3a0a22e1d2657578aae0aa9fcde2ac24fa6a4a2",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libref_array-0.1.5-40.el8.x86_64.rpm"],
)

rpm(
    name = "libseccomp-0__2.5.2-1.el8.x86_64",
    sha256 = "4a6322832274a9507108719de9af48406ee0fcfc54c9906b9450e1ae231ede4b",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libseccomp-2.5.2-1.el8.x86_64.rpm"],
)

rpm(
    name = "libselinux-0__2.9-8.el8.x86_64",
    sha256 = "67f7412ebbbc65ec953aa4e99489c04f821c9645fe048c3ee170040663535dc2",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libselinux-2.9-8.el8.x86_64.rpm"],
)

rpm(
    name = "libselinux-utils-0__2.9-8.el8.x86_64",
    sha256 = "d54bc5c131a6b41d8d69235dcb33ddb8a96df549f3da7b3020bf4dbdee65d71e",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libselinux-utils-2.9-8.el8.x86_64.rpm"],
)

rpm(
    name = "libsemanage-0__2.9-9.el8.x86_64",
    sha256 = "7b8293193b1dda6c408c04074c4b501faf37ff9e4a4b6cd1ca2cce81d5bb67bf",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libsemanage-2.9-9.el8.x86_64.rpm"],
)

rpm(
    name = "libsepol-0__2.9-3.el8.x86_64",
    sha256 = "f91e372ffa25c4c82ae7e001565cf5ff73048c407083493555025fdb5fc4c14a",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libsepol-2.9-3.el8.x86_64.rpm"],
)

rpm(
    name = "libsigsegv-0__2.11-5.el8.x86_64",
    sha256 = "02d728cf74eb47005babeeab5ac68ca04472c643203a1faef0037b5f33710fe2",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libsigsegv-2.11-5.el8.x86_64.rpm"],
)

rpm(
    name = "libsmartcols-0__2.32.1-39.el8.x86_64",
    sha256 = "9277921d36c7164667fb6be5fe191adec82ef6f6e50551ccc7a26c9f3a5cc67b",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libsmartcols-2.32.1-39.el8.x86_64.rpm"],
)

rpm(
    name = "libsoup-0__2.62.3-3.el8.x86_64",
    sha256 = "b97273e313e5234cef54eb7fa9bd12249194b83664e28d6bfd724e69717e9c1f",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libsoup-2.62.3-3.el8.x86_64.rpm"],
)

rpm(
    name = "libssh-0__0.9.6-6.el8.x86_64",
    sha256 = "7a7be0fa0aaa91578c344e708499b2bcb005c1d5c998fb341028e7c00060621e",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libssh-0.9.6-6.el8.x86_64.rpm"],
)

rpm(
    name = "libssh-config-0__0.9.6-6.el8.x86_64",
    sha256 = "1d31c42c9b71d3c2be20f057f71343b44fcb1e5f8d508ef4bdff5484e2c46976",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libssh-config-0.9.6-6.el8.noarch.rpm"],
)

rpm(
    name = "libstdc__plus____plus__-0__8.5.0-18.el8.x86_64",
    sha256 = "91a93afc2dbe65e81d832f3ceec6f84b91e67f9f3026c3def707972ad8eb4b82",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libstdc++-8.5.0-18.el8.x86_64.rpm"],
)

rpm(
    name = "libtasn1-0__4.13-4.el8.x86_64",
    sha256 = "ed93dccf7bf6d8540d825f0021b64164e006ef1c84ba9908d5c57cbb0aef2d8a",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libtasn1-4.13-4.el8.x86_64.rpm"],
)

rpm(
    name = "libthai-0__0.1.27-2.el8.x86_64",
    sha256 = "91bbf9cd7d7ae62682a3d24a889512daef57c3c4b41866336c42af6361702fef",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libthai-0.1.27-2.el8.x86_64.rpm"],
)

rpm(
    name = "libtheora-1__1.1.1-21.el8.x86_64",
    sha256 = "c69987e10c401be766c0a73ade99478d69bad4a2b10688ce9e80295f3f9dae26",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libtheora-1.1.1-21.el8.x86_64.rpm"],
)

rpm(
    name = "libtirpc-0__1.1.4-8.el8.x86_64",
    sha256 = "bcade31f01063824b3a3e77218caaedd16532413282978c437c82b81c2991e4e",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libtirpc-1.1.4-8.el8.x86_64.rpm"],
)

rpm(
    name = "libtpms-0__0.9.1-1.20211126git1ff6fe1f43.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "22948530ccb9782fb07a6fadbe1904e7c8d9863d6f097d3fb210a7b63d4843fd",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libtpms-0.9.1-1.20211126git1ff6fe1f43.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libunistring-0__0.9.9-3.el8.x86_64",
    sha256 = "20bb189228afa589141d9c9d4ed457729d13c11608305387602d0b00ed0a3093",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libunistring-0.9.9-3.el8.x86_64.rpm"],
)

rpm(
    name = "libusbx-0__1.0.23-4.el8.x86_64",
    sha256 = "7e704756a93f07feec345a9748204e78994ce06a4667a2ef35b44964ff754306",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libusbx-1.0.23-4.el8.x86_64.rpm"],
)

rpm(
    name = "libutempter-0__1.1.6-14.el8.x86_64",
    sha256 = "c8c54c56bff9ca416c3ba6bccac483fb66c81a53d93a19420088715018ed5169",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libutempter-1.1.6-14.el8.x86_64.rpm"],
)

rpm(
    name = "libuuid-0__2.32.1-39.el8.x86_64",
    sha256 = "558002d6a6d0369bd68dd2df750149f99db98b0a981769f0f1b21072bc49d189",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libuuid-2.32.1-39.el8.x86_64.rpm"],
)

rpm(
    name = "libverto-0__0.3.2-2.el8.x86_64",
    sha256 = "96b8ea32c5e9b3275788525ecbf35fd6ac1ae137754a2857503776512d4db58a",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libverto-0.3.2-2.el8.x86_64.rpm"],
)

rpm(
    name = "libverto-libev-0__0.3.2-2.el8.x86_64",
    sha256 = "cea0915f850f6fb3b3647302c89c03f8f572c767385b1aaa631b515776182a78",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libverto-libev-0.3.2-2.el8.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "f9f3f072b9eb5264a3da1b29b9dd7ce3dedfb281807ef209011bf5092327c4f7",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-interface-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "ace284c88bbfa6385603f71dce9e305b2fbfabda86abdb2f78b992eff04542bd",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-interface-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-network-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "b5cf9a38e775d527c57e61213f197a8c00924fc6d17319a103df18ad2a2cc2be",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-network-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-nodedev-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "c8f2b9454d937beb497f9006f1ce9c2e4fef0d5b11083c362e6b9f436cf8d720",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-nodedev-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-nwfilter-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "14c5a26a100c9b8d3bf088cebb7bb5053041eef79a1523fb6cbeb8e97e964f59",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-nwfilter-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-qemu-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "66259f3ae0d33d30a70400c6b151d998b30ee38dac87fca2331947e50b9495c7",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-qemu-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-secret-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "5b762f9c227ecf191053cf64944c3a86831eed9ae55fad91398f6950708d221a",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-secret-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-storage-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "1d84046c872615af5a6715d9bc012d9816e5ad6f9fe61ff33cd4b10acc34308f",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-storage-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-storage-core-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "4e8ab4897ed86515f8e80ecb44e00e98a14d18506ee122cf62e672029c262322",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-storage-core-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-storage-disk-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "993dc8e5ec28e5443e119217df68cdf7dcf345ec6809781a07a95d6e4688e3d9",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-storage-disk-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-storage-gluster-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "a1075f247e017c87fabfd940adc5e2b0c0b054a8215a5d73c5b7fd3ccae46bb3",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-storage-gluster-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-storage-iscsi-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "c2d65e1147cbb401c669c3129b41bddf102366164048e4ba5e75ad80947feceb",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-storage-iscsi-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-storage-iscsi-direct-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "9ec1e4f98aacd6317aa0f8d8129e12325145e45b8f11ffe4d4d08678538922f1",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-storage-iscsi-direct-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-storage-logical-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "a32c518d060d85ff5898e5d872eb4c9bf384fd73e94e95b4c6b823d023e8faf0",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-storage-logical-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-storage-mpath-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "6104ed7c457c27e14ef8d399d9592d865f4fa13c58f5555c74c2a483fd18f336",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-storage-mpath-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-storage-rbd-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "a53e731273e074a554552e2cf9c00534acc90b73a2a95c9f87c97aa5438cb930",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-storage-rbd-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-storage-scsi-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "05ecf94112fc9f4ab972f73dc13c88eb45866146baf180f2f161843dfc68c4a4",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-storage-scsi-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-kvm-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "0b534a52db52ca1bf95e22bc9ff2800655433e80164539cccec51c33fde9ce21",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-daemon-kvm-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvirt-libs-0__8.0.0-10.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "11d96648c5531b36aa2af23b4e79955c188027f87067ee685f99c9ce96035db5",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvirt-libs-8.0.0-10.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "libvisual-1__0.4.0-25.el8.x86_64",
    sha256 = "3a95e5f7b43313656f7b5a4798315355457cca2b120a8cfb1883628160fd77c8",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvisual-0.4.0-25.el8.x86_64.rpm"],
)

rpm(
    name = "libvorbis-1__1.3.6-2.el8.x86_64",
    sha256 = "5349766076fcd168287f116b023caa93d451243663b00a5ca5991f74067bf7af",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libvorbis-1.3.6-2.el8.x86_64.rpm"],
)

rpm(
    name = "libwayland-client-0__1.21.0-1.el8.x86_64",
    sha256 = "bf1b7055999f0961fcd23fb29d07678c9d6bf1f9c57f42b06b6237b84a3f5aa9",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libwayland-client-1.21.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "libwayland-cursor-0__1.21.0-1.el8.x86_64",
    sha256 = "ed32158e75e2f3decf8089f5de5dbdf21915c881293a795f5e77cfba3d3af403",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libwayland-cursor-1.21.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "libwayland-egl-0__1.21.0-1.el8.x86_64",
    sha256 = "aa7b2f9d27c75f0844bdbcd02c325aafb79756f1b422fd8d6c229afd4c9c79ad",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libwayland-egl-1.21.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "libwayland-server-0__1.21.0-1.el8.x86_64",
    sha256 = "86b1b725f8b725706cbad9d44d0c896a52b249b3e7b556814128dabc03cef023",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libwayland-server-1.21.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "libxcb-0__1.13.1-1.el8.x86_64",
    sha256 = "0221e6e3671c2bd130e9519a7b352404b7e510584b4707d38e1a733e19c7f74f",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libxcb-1.13.1-1.el8.x86_64.rpm"],
)

rpm(
    name = "libxcrypt-0__4.1.1-6.el8.x86_64",
    sha256 = "645853feb85c921d979cb9cf9109663528429eda63cf5a1e31fe578d3d7e713a",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libxcrypt-4.1.1-6.el8.x86_64.rpm"],
)

rpm(
    name = "libxkbcommon-0__0.9.1-1.el8.x86_64",
    sha256 = "e03d462995326a4477dcebc8c12eae3c1776ce2f095617ace253c0c492c89082",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libxkbcommon-0.9.1-1.el8.x86_64.rpm"],
)

rpm(
    name = "libxml2-0__2.9.7-16.el8.x86_64",
    sha256 = "65d7bffcef57650a109b44992b4b15fa554ce865a0eb21d5ede2aa39f62d4e00",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libxml2-2.9.7-16.el8.x86_64.rpm"],
)

rpm(
    name = "libxshmfence-0__1.3-2.el8.x86_64",
    sha256 = "bfb818e14cfa05d800f1131366ee8fd0c30ab0c735470c870e62dabb7d3f1073",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/libxshmfence-1.3-2.el8.x86_64.rpm"],
)

rpm(
    name = "libxslt-0__1.1.32-6.el8.x86_64",
    sha256 = "250a8077296adcd83585002ff36684be416ba1481d7bd9ed96973e37b9137f00",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libxslt-1.1.32-6.el8.x86_64.rpm"],
)

rpm(
    name = "libyaml-0__0.1.7-5.el8.x86_64",
    sha256 = "00d537a434b1c2896dada83deb359d71fd005772031c73499c72f2cbd34521c5",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libyaml-0.1.7-5.el8.x86_64.rpm"],
)

rpm(
    name = "libzstd-0__1.4.4-1.el8.x86_64",
    sha256 = "7c2dc6044f13fe4ae04a4c1620da822a6be591b5129bf68ba98a3d8e9092f83b",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/libzstd-1.4.4-1.el8.x86_64.rpm"],
)

rpm(
    name = "linux-firmware-0__20220726-110.git150864a4.el8.x86_64",
    sha256 = "6a2b5240aee0494238a20eac9496e7dace3274dbe128eb743803dedfceb47df4",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/linux-firmware-20220726-110.git150864a4.el8.noarch.rpm"],
)

rpm(
    name = "llvm-compat-libs-0__14.0.6-1.module_el8.8.0__plus__1224__plus__64629835.x86_64",
    sha256 = "d5b56f06a379eff11206eaec28a263df36cd8afbb833d57f56320badca590b59",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/llvm-compat-libs-14.0.6-1.module_el8.8.0+1224+64629835.x86_64.rpm"],
)

rpm(
    name = "lua-libs-0__5.3.4-12.el8.x86_64",
    sha256 = "0268af0ee5754fb90fcf71b00fb737f1bf5b3c54c9ff312f13df8c2201311cfe",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/lua-libs-5.3.4-12.el8.x86_64.rpm"],
)

rpm(
    name = "lvm2-8__2.03.14-9.el8.x86_64",
    sha256 = "0cdb3eec6f29415f64e2106d3c27a2797a5c462c4913fa3fc471ae50761144f8",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/lvm2-2.03.14-9.el8.x86_64.rpm"],
)

rpm(
    name = "lvm2-libs-8__2.03.14-9.el8.x86_64",
    sha256 = "2a3692b8f3783dac84b87bddaf9a90c07211cd469851cef8c766f47625513c51",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/lvm2-libs-2.03.14-9.el8.x86_64.rpm"],
)

rpm(
    name = "lz4-libs-0__1.8.3-3.el8_4.x86_64",
    sha256 = "8ecac05bb0ec99f91026f2361f7443b9be3272582193a7836884ec473bf8f423",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/lz4-libs-1.8.3-3.el8_4.x86_64.rpm"],
)

rpm(
    name = "lzo-0__2.08-14.el8.x86_64",
    sha256 = "5c68635cb03533a38d4a42f6547c21a1d5f9952351bb01f3cf865d2621a6e634",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/lzo-2.08-14.el8.x86_64.rpm"],
)

rpm(
    name = "lzop-0__1.03-20.el8.x86_64",
    sha256 = "04eae61018a5be7656be832797016f97cd7b6e19d56f58cb658cd3969dedf2b0",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/lzop-1.03-20.el8.x86_64.rpm"],
)

rpm(
    name = "man-db-0__2.7.6.1-18.el8.x86_64",
    sha256 = "15a21b7abaee01c5f9f443b6dd8e71a6854e10055b7464c68ac7497b1fef5eed",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/man-db-2.7.6.1-18.el8.x86_64.rpm"],
)

rpm(
    name = "mdevctl-0__1.1.0-2.el8.x86_64",
    sha256 = "1d05bf0b9b60c05bece129b9ce3bb3b6e9153ac118d7f347371c4d0cad3f295c",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/mdevctl-1.1.0-2.el8.x86_64.rpm"],
)

rpm(
    name = "mesa-dri-drivers-0__22.3.0-1.el8.x86_64",
    sha256 = "533da3d6b9440a997c8bfb36072ca8cb5a5d880d59d0f249c48bbaf3da77b03c",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/mesa-dri-drivers-22.3.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "mesa-filesystem-0__22.3.0-1.el8.x86_64",
    sha256 = "8621f6e5aa14675722a2c2402fec04f95fa2ab137889593fa017a3d42524e40e",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/mesa-filesystem-22.3.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "mesa-libEGL-0__22.3.0-1.el8.x86_64",
    sha256 = "50ab415cfba02d1d24859ef79a6118d01ffe87dd1cda6b3afb43447847882ce2",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/mesa-libEGL-22.3.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "mesa-libGL-0__22.3.0-1.el8.x86_64",
    sha256 = "34c9c1b4fadc4fdc91fbb05b9c52e2dbd5fc44f014ba2cdd48cd18380717fad1",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/mesa-libGL-22.3.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "mesa-libgbm-0__22.3.0-1.el8.x86_64",
    sha256 = "d23996e6c193cdd4c37971a79dd2b5fbec64ac476ae920b9c49aef91406ba609",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/mesa-libgbm-22.3.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "mesa-libglapi-0__22.3.0-1.el8.x86_64",
    sha256 = "dbdaa8e1d30a19c0e60a2dd4848d170cf3e5714a5f7166ee89035ccbfeb4b5dc",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/mesa-libglapi-22.3.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "mozjs60-0__60.9.0-4.el8.x86_64",
    sha256 = "03b50a4ea5cf5655c67e2358fabb6e563eec4e7929e7fc6c4e92c92694f60fa0",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/mozjs60-60.9.0-4.el8.x86_64.rpm"],
)

rpm(
    name = "mpfr-0__3.1.6-1.el8.x86_64",
    sha256 = "e7f0c34f83c1ec2abb22951779e84d51e234c4ba0a05252e4ffd8917461891a5",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/mpfr-3.1.6-1.el8.x86_64.rpm"],
)

rpm(
    name = "nbdkit-0__1.24.0-4.module_el8.6.0__plus__1087__plus__b42c8331.x86_64",
    sha256 = "4892cc02d2771c1aa41362261199775c5916a8a5c077fa18fdd00ad422b96d9b",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nbdkit-1.24.0-4.module_el8.6.0+1087+b42c8331.x86_64.rpm"],
)

rpm(
    name = "nbdkit-basic-filters-0__1.24.0-4.module_el8.6.0__plus__1087__plus__b42c8331.x86_64",
    sha256 = "1a7c21176ae2d944c9b5e3c45c9d76780c7fe845d944cb6273681db702f6756d",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nbdkit-basic-filters-1.24.0-4.module_el8.6.0+1087+b42c8331.x86_64.rpm"],
)

rpm(
    name = "nbdkit-basic-plugins-0__1.24.0-4.module_el8.6.0__plus__1087__plus__b42c8331.x86_64",
    sha256 = "8ccbb8e40df10dcfa8a4ce4725390f1d65ed5e5031a84c02ae187258df281cef",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nbdkit-basic-plugins-1.24.0-4.module_el8.6.0+1087+b42c8331.x86_64.rpm"],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.24.0-4.module_el8.6.0__plus__1087__plus__b42c8331.x86_64",
    sha256 = "73eb1576459eb64b09f1a6fd22f95fcee5776ffa1e121e8bd0669c687d2e0947",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nbdkit-curl-plugin-1.24.0-4.module_el8.6.0+1087+b42c8331.x86_64.rpm"],
)

rpm(
    name = "nbdkit-python-plugin-0__1.24.0-4.module_el8.6.0__plus__1087__plus__b42c8331.x86_64",
    sha256 = "3c8f976a390d291058af429dba2e8cc34759be0c32001c43743eadc251abd07f",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nbdkit-python-plugin-1.24.0-4.module_el8.6.0+1087+b42c8331.x86_64.rpm"],
)

rpm(
    name = "nbdkit-server-0__1.24.0-4.module_el8.6.0__plus__1087__plus__b42c8331.x86_64",
    sha256 = "542b3eba91abbec51b83d4a3cb21c8236342462f5742784a6fef8dc0740d8837",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nbdkit-server-1.24.0-4.module_el8.6.0+1087+b42c8331.x86_64.rpm"],
)

rpm(
    name = "nbdkit-ssh-plugin-0__1.24.0-4.module_el8.6.0__plus__1087__plus__b42c8331.x86_64",
    sha256 = "1c9869e1914b30f529763c88530101f2e95b57a8ba9700a9b5dcf986642c4017",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nbdkit-ssh-plugin-1.24.0-4.module_el8.6.0+1087+b42c8331.x86_64.rpm"],
)

rpm(
    name = "nbdkit-vddk-plugin-0__1.24.0-4.module_el8.6.0__plus__1087__plus__b42c8331.x86_64",
    sha256 = "f5f6379ff42015c3d0ce602d9dd34f5db19af6e66928f2018a3f639ac24a3f95",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nbdkit-vddk-plugin-1.24.0-4.module_el8.6.0+1087+b42c8331.x86_64.rpm"],
)

rpm(
    name = "ncurses-base-0__6.1-9.20180224.el8.x86_64",
    sha256 = "41716536ea16798238ac89fbc3041b3f9dc80f9a64ea4b19d6e67ad2c909269a",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/ncurses-base-6.1-9.20180224.el8.noarch.rpm"],
)

rpm(
    name = "ncurses-libs-0__6.1-9.20180224.el8.x86_64",
    sha256 = "54609dd070a57a14a6103f0c06bea99bb0a4e568d1fbc6a22b8ba67c954d90bf",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/ncurses-libs-6.1-9.20180224.el8.x86_64.rpm"],
)

rpm(
    name = "ndctl-libs-0__71.1-4.el8.x86_64",
    sha256 = "d1518d8f29a72c8c9501f67929258405cf25fd4be365fd905acc57b846d49c8a",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/ndctl-libs-71.1-4.el8.x86_64.rpm"],
)

rpm(
    name = "netcf-libs-0__0.2.8-12.module_el8.6.0__plus__983__plus__a7505f3f.x86_64",
    sha256 = "e0a16e40b6cc6c803d2f1e49245d5b9000d915aa02663106cbd4195c99efdd56",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/netcf-libs-0.2.8-12.module_el8.6.0+983+a7505f3f.x86_64.rpm"],
)

rpm(
    name = "nettle-0__3.4.1-7.el8.x86_64",
    sha256 = "fe9a848502c595e0b7acc699d69c24b9c5ad0ac58a0b3933cd228f3633de31cb",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/nettle-3.4.1-7.el8.x86_64.rpm"],
)

rpm(
    name = "nfs-utils-1__2.3.3-59.el8.x86_64",
    sha256 = "7738ef840fc85aa684dd8475c0d036c87744d9d2db271deb7b49c60042ec9358",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/nfs-utils-2.3.3-59.el8.x86_64.rpm"],
)

rpm(
    name = "npth-0__1.5-4.el8.x86_64",
    sha256 = "168ab5dbc86b836b8742b2e63eee51d074f1d790728e3d30b0c59fff93cf1d8d",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/npth-1.5-4.el8.x86_64.rpm"],
)

rpm(
    name = "nspr-0__4.34.0-3.el8.x86_64",
    sha256 = "d6bc88f314523b6929f6ef757395fe7a50ce240355c2dc701dfd34869b01f450",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nspr-4.34.0-3.el8.x86_64.rpm"],
)

rpm(
    name = "nss-0__3.79.0-10.el8.x86_64",
    sha256 = "9492fcffdaaaa488d3cf2a90849c0485aa8ff5450cfd64493f4bbff6909a33ec",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nss-3.79.0-10.el8.x86_64.rpm"],
)

rpm(
    name = "nss-softokn-0__3.79.0-10.el8.x86_64",
    sha256 = "ebf262bfcc94da3c9927dd9c37a7e32001233d2b5dc8a09d713f257c30162dd2",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nss-softokn-3.79.0-10.el8.x86_64.rpm"],
)

rpm(
    name = "nss-softokn-freebl-0__3.79.0-10.el8.x86_64",
    sha256 = "2d9b9c9e6ffe2ce000bea782233c778e6517aea7fa359122c6f69415cdc6226c",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nss-softokn-freebl-3.79.0-10.el8.x86_64.rpm"],
)

rpm(
    name = "nss-sysinit-0__3.79.0-10.el8.x86_64",
    sha256 = "65bcaf16753f7c19d05e4ff11aa2eb4d8dc72c807298cca74c593e8847585944",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nss-sysinit-3.79.0-10.el8.x86_64.rpm"],
)

rpm(
    name = "nss-util-0__3.79.0-10.el8.x86_64",
    sha256 = "a107a113e0960acd666be18ec1ef1c4cee770b19542e84a16b418914b6e62532",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/nss-util-3.79.0-10.el8.x86_64.rpm"],
)

rpm(
    name = "numactl-libs-0__2.0.12-13.el8.x86_64",
    sha256 = "b7b71ba34b3af893dc0acbb9d2228a2307da849d38e1c0007bd3d64f456640af",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/numactl-libs-2.0.12-13.el8.x86_64.rpm"],
)

rpm(
    name = "numad-0__0.5-26.20150602git.el8.x86_64",
    sha256 = "5d975c08273b1629683275c32f16e52ca8e37e6836598e211092c915d38878bf",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/numad-0.5-26.20150602git.el8.x86_64.rpm"],
)

rpm(
    name = "openldap-0__2.4.46-18.el8.x86_64",
    sha256 = "95327d6c83a370a12c125767403496435d20a94b70ee395eabfc356270d2ada9",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/openldap-2.4.46-18.el8.x86_64.rpm"],
)

rpm(
    name = "openssl-libs-1__1.1.1k-7.el8.x86_64",
    sha256 = "7b42ba3855f29955fe204ad7c189a832a5b1423a32abcda079d8ef2f787c8e73",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/openssl-libs-1.1.1k-7.el8.x86_64.rpm"],
)

rpm(
    name = "opus-0__1.3-0.4.beta.el8.x86_64",
    sha256 = "00512c56e8931eb0ab52de91d0272f00bf904d6f2042b580115edd7eb4a42df2",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/opus-1.3-0.4.beta.el8.x86_64.rpm"],
)

rpm(
    name = "orc-0__0.4.28-3.el8.x86_64",
    sha256 = "7552ad64b02a15a3b91524f9858afeb228ef45148204539ad33524f7d7bc5c67",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/orc-0.4.28-3.el8.x86_64.rpm"],
)

rpm(
    name = "osinfo-db-0__20220727-2.el8.x86_64",
    sha256 = "c32bcd38505ee9dd5175665bd300fb45a6f42ab31e24ede3db2fc04ec16a7475",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/osinfo-db-20220727-2.el8.noarch.rpm"],
)

rpm(
    name = "osinfo-db-tools-0__1.9.0-1.el8.x86_64",
    sha256 = "cda8b7779704c6b81148313ac9423694e39e02a243b804b3bfdcace21b053551",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/osinfo-db-tools-1.9.0-1.el8.x86_64.rpm"],
)

rpm(
    name = "p11-kit-0__0.23.22-1.el8.x86_64",
    sha256 = "6a67c8721fe24af25ec56c6aae956a190d8463e46efed45adfbbd800086550c7",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/p11-kit-0.23.22-1.el8.x86_64.rpm"],
)

rpm(
    name = "p11-kit-trust-0__0.23.22-1.el8.x86_64",
    sha256 = "d218619a4859e002fe677703bc1767986314cd196ae2ac397ed057f3bec36516",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/p11-kit-trust-0.23.22-1.el8.x86_64.rpm"],
)

rpm(
    name = "pam-0__1.3.1-25.el8.x86_64",
    sha256 = "1dd647b181f70dfa8a3e742a9942f3b134c17a721f890057b756691f2389333c",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/pam-1.3.1-25.el8.x86_64.rpm"],
)

rpm(
    name = "pango-0__1.42.4-8.el8.x86_64",
    sha256 = "1e74c391edf2f383b5c236e65ddd15bcf83883975b8d08b70808d2e14916d496",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/pango-1.42.4-8.el8.x86_64.rpm"],
)

rpm(
    name = "parted-0__3.2-39.el8.x86_64",
    sha256 = "2a9f8558c6c640d8f035004f3a9e607f6941e028785da562f01b61a142b5e282",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/parted-3.2-39.el8.x86_64.rpm"],
)

rpm(
    name = "pcre-0__8.42-6.el8.x86_64",
    sha256 = "876e9e99b0e50cb2752499045bafa903dd29e5c491d112daacef1ae16f614dad",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/pcre-8.42-6.el8.x86_64.rpm"],
)

rpm(
    name = "pcre2-0__10.32-3.el8.x86_64",
    sha256 = "2f865747024d26b91d5a9f2f35dd1b04e1039d64e772d0371b437145cd7beceb",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/pcre2-10.32-3.el8.x86_64.rpm"],
)

rpm(
    name = "pixman-0__0.38.4-2.el8.x86_64",
    sha256 = "e496740940bd0b4d6f6537feaaffff57580624f6629c736c7f5e415259dc6cbe",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/pixman-0.38.4-2.el8.x86_64.rpm"],
)

rpm(
    name = "platform-python-0__3.6.8-51.el8.x86_64",
    sha256 = "9958ab63b5f061c9b1e2c3cfd4d0f26166a2abb0914a8c556be7d7159063905c",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/platform-python-3.6.8-51.el8.x86_64.rpm"],
)

rpm(
    name = "platform-python-setuptools-0__39.2.0-7.el8.x86_64",
    sha256 = "e7b5b0904239cf0eaed16cbec17825fee9465c700de385a1ceb87db671c4bce7",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/platform-python-setuptools-39.2.0-7.el8.noarch.rpm"],
)

rpm(
    name = "policycoreutils-0__2.9-21.1.el8.x86_64",
    sha256 = "b469a4d6f9955cc06d37e9931fd24cb842a8afd2589a97f16d93a7d4e8dc5885",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/policycoreutils-2.9-21.1.el8.x86_64.rpm"],
)

rpm(
    name = "policycoreutils-python-utils-0__2.9-21.1.el8.x86_64",
    sha256 = "7fb15c5775eb6f35720750b5754a957ba6f2adef662c108f4d12d09bee0b9a7f",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/policycoreutils-python-utils-2.9-21.1.el8.noarch.rpm"],
)

rpm(
    name = "polkit-0__0.115-13.0.1.el8.2.x86_64",
    sha256 = "8bfccf9235747eb132c1d10c2f26b5544a0db078019eb7911b88522131e16dc8",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/polkit-0.115-13.0.1.el8.2.x86_64.rpm"],
)

rpm(
    name = "polkit-libs-0__0.115-13.0.1.el8.2.x86_64",
    sha256 = "d957da6b452f7b15830ad9a73176d4f04d9c3e26e119b7f3f4f4060087bb9082",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/polkit-libs-0.115-13.0.1.el8.2.x86_64.rpm"],
)

rpm(
    name = "polkit-pkla-compat-0__0.1-12.el8.x86_64",
    sha256 = "e7ee4b6d6456cb7da0332f5a6fb8a7c47df977bcf616f12f0455413765367e89",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/polkit-pkla-compat-0.1-12.el8.x86_64.rpm"],
)

rpm(
    name = "popt-0__1.18-1.el8.x86_64",
    sha256 = "3fc009f00388e66befab79be548ff3c7aa80ca70bd7f183d22f59137d8e2c2ae",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/popt-1.18-1.el8.x86_64.rpm"],
)

rpm(
    name = "procps-ng-0__3.3.15-11.el8.x86_64",
    sha256 = "c051ea0fae01a366ff0625e59d27037a88bdb7d1640e91dc8dc5c531275bd831",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/procps-ng-3.3.15-11.el8.x86_64.rpm"],
)

rpm(
    name = "psmisc-0__23.1-5.el8.x86_64",
    sha256 = "9d433d8c058e59c891c0852b95b3b87795ea30a85889c77ba0b12f965517d626",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/psmisc-23.1-5.el8.x86_64.rpm"],
)

rpm(
    name = "python3-audit-0__3.0.7-4.el8.x86_64",
    sha256 = "9b1b099aba60b188b29dae983994dce70a0c5887f75f0e1b1c794e95868fb6e2",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/python3-audit-3.0.7-4.el8.x86_64.rpm"],
)

rpm(
    name = "python3-libs-0__3.6.8-51.el8.x86_64",
    sha256 = "19f1ab05fbb9723793e263a5e5e5bb2f03cb88e67ef672310cb7abf1cc2c8d0b",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/python3-libs-3.6.8-51.el8.x86_64.rpm"],
)

rpm(
    name = "python3-libselinux-0__2.9-8.el8.x86_64",
    sha256 = "72ec65891ea01feb1ebb88e38f34fda4ed4faa62ef30d14d76ea5edfc1822378",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/python3-libselinux-2.9-8.el8.x86_64.rpm"],
)

rpm(
    name = "python3-libsemanage-0__2.9-9.el8.x86_64",
    sha256 = "ec6f98ebfe2588a4e7076dcee13c939e0c883b5228d761701eb544d4692478d7",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/python3-libsemanage-2.9-9.el8.x86_64.rpm"],
)

rpm(
    name = "python3-pip-wheel-0__9.0.3-22.el8.x86_64",
    sha256 = "772093492e290af496c3c8d4cf1d83d3288af49c4f0eb550f9c2489f96ecd89d",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/python3-pip-wheel-9.0.3-22.el8.noarch.rpm"],
)

rpm(
    name = "python3-policycoreutils-0__2.9-21.1.el8.x86_64",
    sha256 = "90ac836d4fe6b2e881e9c05e1d1acfc429423cdb35375792e15a7328dfcef76e",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/python3-policycoreutils-2.9-21.1.el8.noarch.rpm"],
)

rpm(
    name = "python3-pyyaml-0__3.12-12.el8.x86_64",
    sha256 = "525393e4d658e395c6280bd2ff4afe54999796c4722986325297ba4bfade3ea5",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/python3-pyyaml-3.12-12.el8.x86_64.rpm"],
)

rpm(
    name = "python3-setools-0__4.3.0-3.el8.x86_64",
    sha256 = "9851a70ab1371b4e86cdd268d36a7a87266c915fe7cb8a59ea8d422df320febf",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/python3-setools-4.3.0-3.el8.x86_64.rpm"],
)

rpm(
    name = "python3-setuptools-wheel-0__39.2.0-7.el8.x86_64",
    sha256 = "202a208dc9390ef3fd1528100fb80059970cfcc2698b5aaa8896f710d30b61e0",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/python3-setuptools-wheel-39.2.0-7.el8.noarch.rpm"],
)

rpm(
    name = "qemu-guest-agent-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "9ea98288d0991f3fa2af26d0ec18d9a839f05c5682252e70b781989d6eb27e6f",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-guest-agent-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "qemu-img-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "9bbb3b752390de1d16924ccbc9b124b4582fa91ecfeaf0b8fd143b2e4f434cdb",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-img-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "fa4918aa9f6a5de8b03fd0805f7b4ad0e998aa5d7ba1caacef943b11058e2c89",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-kvm-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-block-curl-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "e440b324e780ace47723664fb27dab36fc9a007f0c1204a727b721ada81035b1",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-kvm-block-curl-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-block-gluster-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "aeeb370c1bb21f90d2f7dc68089e90a15528e240a02bfe3eb96af8bc48cc3a2c",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-kvm-block-gluster-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-block-iscsi-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "f0bf9663e03b7bd2eecc8e8032b63c4e6916318659af145aacd17115690b916a",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-kvm-block-iscsi-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-block-rbd-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "746197475a3cf9c9a667ba5dcda99ba780ba30ff581d8e1b7b4d1953284584b1",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-kvm-block-rbd-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-block-ssh-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "77813bd94ab0604a2a13d06d76a1246d431ced2839797048f9eefc7ec02aee94",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-kvm-block-ssh-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-common-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "855ac910b92265ba5100f9ca4373935ac9aa44bd53f5af29d39add79a210daf6",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-kvm-common-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-core-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "ac04b3b8f64583e2081add6e5b6a64e213c548b6fd41551bb5afe7dc89dcac2e",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-kvm-core-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-docs-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "76ae36353dbe131c73342c8b013262127b4da21cf7258329ceca34699b86a1b4",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-kvm-docs-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-hw-usbredir-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "c7a9293083835cf3cb7f0b706786983271ef334c4e9dc2212707952ac7d01e97",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-kvm-hw-usbredir-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-ui-opengl-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "db98f2e613cea8517c65900947b5450d61d1bb9765188428a00f3ab7d5d69042",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-kvm-ui-opengl-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-ui-spice-15__6.2.0-20.module_el8.7.0__plus__1218__plus__f626c2ff.1.x86_64",
    sha256 = "767aa185b149259eee442f12d9eebe1f6e4895134e2982c461dcd34244054193",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/qemu-kvm-ui-spice-6.2.0-20.module_el8.7.0+1218+f626c2ff.1.x86_64.rpm"],
)

rpm(
    name = "quota-1__4.04-14.el8.x86_64",
    sha256 = "cce5f4086e7ecc31a12b753b5d0d97cb6d6c6f61e5c3066322449781ab1f63d0",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/quota-4.04-14.el8.x86_64.rpm"],
)

rpm(
    name = "quota-nls-1__4.04-14.el8.x86_64",
    sha256 = "bc7fc2028a29ac7a406719ed4f6740f6bf12c20961223c1e839a2a39069af38d",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/quota-nls-4.04-14.el8.noarch.rpm"],
)

rpm(
    name = "readline-0__7.0-10.el8.x86_64",
    sha256 = "fea868a7d82a7b6f392260ed4afb472dc4428fd71eab1456319f423a845b5084",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/readline-7.0-10.el8.x86_64.rpm"],
)

rpm(
    name = "rpcbind-0__1.2.5-10.el8.x86_64",
    sha256 = "33100cb3945ea696c0b942691a34a3a2a205944f84327884235d8782d20e21bc",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/rpcbind-1.2.5-10.el8.x86_64.rpm"],
)

rpm(
    name = "rpm-0__4.14.3-26.el8.x86_64",
    sha256 = "453a504eb33a2d1fd337a8465bc251a6623bab10b82af866f300374c32519588",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/rpm-4.14.3-26.el8.x86_64.rpm"],
)

rpm(
    name = "rpm-libs-0__4.14.3-26.el8.x86_64",
    sha256 = "7b62e239e21f1e885e7b6e51ce4ad9ff3e4f3bf2a00f1c5b6bb4bd4eee97ebe3",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/rpm-libs-4.14.3-26.el8.x86_64.rpm"],
)

rpm(
    name = "rpm-plugin-selinux-0__4.14.3-26.el8.x86_64",
    sha256 = "decce24ccc350e080c454f2f672d35d343bb4251930c84fa698f38f0110a6a64",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/rpm-plugin-selinux-4.14.3-26.el8.x86_64.rpm"],
)

rpm(
    name = "seabios-bin-0__1.16.0-3.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "92edef92725941ce3a90551380ff0792486e8a2f11e6ceacbf420bbc12460ab3",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/seabios-bin-1.16.0-3.module_el8.7.0+1218+f626c2ff.noarch.rpm"],
)

rpm(
    name = "seavgabios-bin-0__1.16.0-3.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "00156d667665c88c1e87d913d966a070d9eee2c7609f99e98d43758b75d19ee8",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/seavgabios-bin-1.16.0-3.module_el8.7.0+1218+f626c2ff.noarch.rpm"],
)

rpm(
    name = "sed-0__4.5-5.el8.x86_64",
    sha256 = "5a09d6d967d12580c7e6ab92db35bcafd3426d6121ec60c78f54e3cd4961cd26",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/sed-4.5-5.el8.x86_64.rpm"],
)

rpm(
    name = "selinux-policy-0__3.14.3-114.el8.x86_64",
    sha256 = "671135b5be5b50b5a88678e34b1697245f9d38b563410652a9fcec7ca1a3af20",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/selinux-policy-3.14.3-114.el8.noarch.rpm"],
)

rpm(
    name = "selinux-policy-targeted-0__3.14.3-114.el8.x86_64",
    sha256 = "46415d695d7418b895a3ea65e511c212701a6df5d9549548ecc0901c18802dc3",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/selinux-policy-targeted-3.14.3-114.el8.noarch.rpm"],
)

rpm(
    name = "setup-0__2.12.2-9.el8.x86_64",
    sha256 = "0a0696aebfadbbeb229445c0828a83be763460d6af6a552b3bd533acde011644",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/setup-2.12.2-9.el8.noarch.rpm"],
)

rpm(
    name = "sgabios-bin-1__0.20170427git-3.module_el8.6.0__plus__983__plus__a7505f3f.x86_64",
    sha256 = "79675eae8221b4abd2ef195328fc9b2c27b7f6e901ed65ac11b93f0637033b2f",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/sgabios-bin-0.20170427git-3.module_el8.6.0+983+a7505f3f.noarch.rpm"],
)

rpm(
    name = "shadow-utils-2__4.6-17.el8.x86_64",
    sha256 = "fb3c71778fc23c4d3c91911c49e0a0d14c8a5192c431fc9ba07f2a14c938a172",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/shadow-utils-4.6-17.el8.x86_64.rpm"],
)

rpm(
    name = "snappy-0__1.1.8-3.el8.x86_64",
    sha256 = "839c62cd7fc7e152decded6f28c80b5f7b8f34a5e319057867b38b26512cee67",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/snappy-1.1.8-3.el8.x86_64.rpm"],
)

rpm(
    name = "spice-server-0__0.14.3-4.el8.x86_64",
    sha256 = "1dea958ebe37b61062fd7313234b41628ad68de34dd1b615df3f42b7975ecb6b",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/spice-server-0.14.3-4.el8.x86_64.rpm"],
)

rpm(
    name = "sqlite-libs-0__3.26.0-17.el8.x86_64",
    sha256 = "a44b1bd3d9f5a6b0654ba4ae2f8aa45aefec54c9377dfe4446ec1c0e2fd0ac89",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/sqlite-libs-3.26.0-17.el8.x86_64.rpm"],
)

rpm(
    name = "swtpm-0__0.7.0-4.20211109gitb79fd91.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "2125c4d6cb910e47daf45fbef10d75f93b5d30e64908b42dfc77aeee201feb60",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/swtpm-0.7.0-4.20211109gitb79fd91.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "swtpm-libs-0__0.7.0-4.20211109gitb79fd91.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "f29e2f9e3f3c4ba3cddbe4af4dc7db2e7ad0088db6e955da86dacb40d4e75466",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/swtpm-libs-0.7.0-4.20211109gitb79fd91.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "swtpm-tools-0__0.7.0-4.20211109gitb79fd91.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "bb88081e4d8978aaea3e902252be225211fc496f053ac721757a8b005c3ad86d",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/swtpm-tools-0.7.0-4.20211109gitb79fd91.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "systemd-0__239-70.el8.x86_64",
    sha256 = "664732121b2325bfee0d524d4947a2bb82860767d19a131298e47da5e750138c",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/systemd-239-70.el8.x86_64.rpm"],
)

rpm(
    name = "systemd-container-0__239-70.el8.x86_64",
    sha256 = "63331ca31a1d45ede3f78c95a17cb32021c0c61342e615a53c249783d079c154",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/systemd-container-239-70.el8.x86_64.rpm"],
)

rpm(
    name = "systemd-libs-0__239-70.el8.x86_64",
    sha256 = "cfa01a0fa8cf10190f7cf1efe2f45f77ba2b11214e98c639951837fe34422696",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/systemd-libs-239-70.el8.x86_64.rpm"],
)

rpm(
    name = "systemd-pam-0__239-70.el8.x86_64",
    sha256 = "da594f12c38ef5c5f10c81c7c18212e7a5aad8b764aaa511dde4389858d42586",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/systemd-pam-239-70.el8.x86_64.rpm"],
)

rpm(
    name = "systemd-udev-0__239-70.el8.x86_64",
    sha256 = "bebff1d796265dccbc9e83875b99559ed4b4c547f912ee7604aa0cca7e702f71",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/systemd-udev-239-70.el8.x86_64.rpm"],
)

rpm(
    name = "tzdata-0__2022g-2.el8.x86_64",
    sha256 = "e20560e5993ea03732b361f1df4fb7bdddf5f62333b4857a94fd1edb5473d691",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/tzdata-2022g-2.el8.noarch.rpm"],
)

rpm(
    name = "unbound-libs-0__1.16.2-5.el8.x86_64",
    sha256 = "50cdd79fd25f9ec2c350c0572b982f896d5b9e52b778b9b4022509d833e894ec",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/unbound-libs-1.16.2-5.el8.x86_64.rpm"],
)

rpm(
    name = "unzip-0__6.0-46.el8.x86_64",
    sha256 = "13a56592b8870bfa141d8fbc4d32b780967a49e7127b93348845be41dd7160a4",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/unzip-6.0-46.el8.x86_64.rpm"],
)

rpm(
    name = "usbredir-0__0.12.0-2.el8.x86_64",
    sha256 = "0b6e50e9e9c68d0dbacc39e81c4a3a3a7ccf3afaddf40afb06ca86424a46ba23",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/usbredir-0.12.0-2.el8.x86_64.rpm"],
)

rpm(
    name = "userspace-rcu-0__0.10.1-4.el8.x86_64",
    sha256 = "4025900345c5125fd6c10c1780275139f56b63be2bfac10be83628758c225dd0",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/userspace-rcu-0.10.1-4.el8.x86_64.rpm"],
)

rpm(
    name = "util-linux-0__2.32.1-39.el8.x86_64",
    sha256 = "071b1a3a157faed2cfb9a48ca0e43cda41ae9cebfd74926f68ccaa379497f278",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/util-linux-2.32.1-39.el8.x86_64.rpm"],
)

rpm(
    name = "vim-minimal-2__8.0.1763-19.el8.4.x86_64",
    sha256 = "8d1659cf14095e2a82da7b2b7c21e5b62fda058590ea66b9e3d33a6794449e2c",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/vim-minimal-8.0.1763-19.el8.4.x86_64.rpm"],
)

rpm(
    name = "virt-v2v-1__1.42.0-21.module_el8.7.0__plus__1218__plus__f626c2ff.x86_64",
    sha256 = "f4b4fe6a2f8f4e2e85038ee1f07e509f8456918e7b2200ccd4a18592e9b825ca",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/virt-v2v-1.42.0-21.module_el8.7.0+1218+f626c2ff.x86_64.rpm"],
)

rpm(
    name = "virtio-win-0__1.9.24-2.el8_5.x86_64",
    sha256 = "395b8bbf79e44590ce7ce340182192d780469ea857a03be91997318303a0db24",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/virtio-win-1.9.24-2.el8_5.noarch.rpm"],
)

rpm(
    name = "which-0__2.21-18.el8.x86_64",
    sha256 = "0e4d5ee4cbea952903ee4febb1450caf92bf3c2d6ecac9d0dd8ac8611e9ff4db",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/which-2.21-18.el8.x86_64.rpm"],
)

rpm(
    name = "wqy-microhei-fonts-0__0.2.0-0.22.beta.el8.x86_64",
    sha256 = "2104b702e6abdf9a59a363acf6f00816679d41d539251ec9a47894b147f38a52",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/wqy-microhei-fonts-0.2.0-0.22.beta.el8.noarch.rpm"],
)

rpm(
    name = "xkeyboard-config-0__2.28-1.el8.x86_64",
    sha256 = "a2aeabb3962859069a78acc288bc3bffb35485428e162caafec8134f5ce6ca67",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/xkeyboard-config-2.28-1.el8.noarch.rpm"],
)

rpm(
    name = "xml-common-0__0.6.3-50.el8.x86_64",
    sha256 = "6d7676847b3c0dbac22983c85c0a419af43029cc3b8ff5dc26c9f85174fc85d8",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/xml-common-0.6.3-50.el8.noarch.rpm"],
)

rpm(
    name = "xz-0__5.2.4-4.el8.x86_64",
    sha256 = "99d7d4bfee1d5b55e08ee27c6869186531939f399d6c3ea33db191cae7e53f70",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/xz-5.2.4-4.el8.x86_64.rpm"],
)

rpm(
    name = "xz-libs-0__5.2.4-4.el8.x86_64",
    sha256 = "69d67ea8b4bd532f750ff0592f0098ace60470da0fd0e4056188fda37a268d42",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/xz-libs-5.2.4-4.el8.x86_64.rpm"],
)

rpm(
    name = "yajl-0__2.1.0-11.el8.x86_64",
    sha256 = "55a094ffe9f378ef465619bf6f60e9f26b672f67236883565fb893de7675c163",
    urls = ["http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/Packages/yajl-2.1.0-11.el8.x86_64.rpm"],
)

rpm(
    name = "zlib-0__1.2.11-21.el8.x86_64",
    sha256 = "9aabeb4a75c05b98661200dc9f0f1c7c528af42b9535c7c133dd4c0c5f80d179",
    urls = ["http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/Packages/zlib-1.2.11-21.el8.x86_64.rpm"],
)
