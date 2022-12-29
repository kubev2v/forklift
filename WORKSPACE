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
    name = "ubi8-minimal",
    # 'tag' is also supported, but digest is encouraged for reproducibility.
    digest = "sha256:d1f8eff6032334a81d7cbfd73dacee680e8138db57ecbc91548b97bb45e698e5",
    registry = "registry.access.redhat.com",
    repository = "ubi8/ubi-minimal",
)

container_pull(
    name = "centos-stream8",
    # 'tag' is also supported, but digest is encouraged for reproducibility.
    digest = "sha256:925b59f6a8f93b4cea34ca7e4d8cacb40784cc12161e0c41081792a71d91221c",
    registry = "quay.io",
    repository = "centos/centos",
    #tag = "stream8",
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

container_pull(
    name = "ubi9-go-toolset",
    digest = "sha256:6fd8a2bd6bc39a15ffe60f89556cec3a2c550cc4d0778bc62f22fc681b5dcd81",
    registry = "registry.access.redhat.com",
    repository = "ubi9/go-toolset",
    tag = "1.18.4-11.1669637104",
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
    name = "acl-0__2.3.1-3.el9.x86_64",
    sha256 = "986044c3837eddbc9231d7be5e5fc517e245296978b988a803bc9f9172fe84ea",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/acl-2.3.1-3.el9.x86_64.rpm"],
)

rpm(
    name = "adobe-source-code-pro-fonts-0__2.030.1.050-12.el9.1.x86_64",
    sha256 = "9e6aa0c60204bb4b152ce541ca3a9f5c28b020ed551dd417d3936a8b2153f0df",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/adobe-source-code-pro-fonts-2.030.1.050-12.el9.1.noarch.rpm"],
)

rpm(
    name = "alternatives-0__1.20-2.el9.x86_64",
    sha256 = "1851d5f64ebaeac67c5c2d9e4adc1e73aa6433b44a167268a3510c3d056062db",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/alternatives-1.20-2.el9.x86_64.rpm"],
)

rpm(
    name = "audit-libs-0__3.0.7-103.el9.x86_64",
    sha256 = "cdd16764f76df434a731a331577fb03a51f19d0a8249ae782506e5ac12dabb0a",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/audit-libs-3.0.7-103.el9.x86_64.rpm"],
)

rpm(
    name = "augeas-libs-0__1.13.0-3.el9.x86_64",
    sha256 = "f15b57d9629d67b29072782d540eb9ca4f89cac4f49de517afd8a0bb4f7ae025",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/augeas-libs-1.13.0-3.el9.x86_64.rpm"],
)

rpm(
    name = "basesystem-0__11-13.el9.x86_64",
    sha256 = "a7a687ef39dd28d01d34fab18ea7e3e87f649f6c202dded82260b7ea625b9973",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/basesystem-11-13.el9.noarch.rpm"],
)

rpm(
    name = "bash-0__5.1.8-6.el9.x86_64",
    sha256 = "09f700a94e187a74f6f4a5f750082732e193d41392a85f042bdeb0bcbabe0a1f",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/bash-5.1.8-6.el9.x86_64.rpm"],
)

rpm(
    name = "boost-iostreams-0__1.75.0-8.el9.x86_64",
    sha256 = "cc7501a1aeb2614969cede7be5cab566aa0b4aa774af56196bdd4fc24f583937",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/boost-iostreams-1.75.0-8.el9.x86_64.rpm"],
)

rpm(
    name = "boost-system-0__1.75.0-8.el9.x86_64",
    sha256 = "6339992cd9222414178311ed96865d9364ed8e4ded63f936b7582fac5df74610",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/boost-system-1.75.0-8.el9.x86_64.rpm"],
)

rpm(
    name = "boost-thread-0__1.75.0-8.el9.x86_64",
    sha256 = "24e9280a94c0fa8652db50f1fb0bf0b26aeb0c249fcd1b662fa8d29d37fe519b",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/boost-thread-1.75.0-8.el9.x86_64.rpm"],
)

rpm(
    name = "bzip2-0__1.0.8-8.el9.x86_64",
    sha256 = "90aeb088fad0093b1ca531387d38e1c32ad64efd56f2306eacc0edbc4c37e205",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/bzip2-1.0.8-8.el9.x86_64.rpm"],
)

rpm(
    name = "bzip2-libs-0__1.0.8-8.el9.x86_64",
    sha256 = "fabd6b5c065c2b9d4a8d39a938ae577d801de2ddc73c8cdf6f7803db29c28d0a",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/bzip2-libs-1.0.8-8.el9.x86_64.rpm"],
)

rpm(
    name = "ca-certificates-0__2022.2.54-90.2.el9.x86_64",
    sha256 = "24978e8dd3e054583da86036657ab16e93da97a0bafc148ec28d871d8c15257c",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ca-certificates-2022.2.54-90.2.el9.noarch.rpm"],
)

rpm(
    name = "capstone-0__4.0.2-10.el9.x86_64",
    sha256 = "f6a9fdc6bcb5da1b2ce44ca7ed6289759c37add7adbb19916dd36d5bb4624a41",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/capstone-4.0.2-10.el9.x86_64.rpm"],
)

rpm(
    name = "centos-gpg-keys-0__9.0-18.el9.x86_64",
    sha256 = "4d08c97e3852712e5a46d37e1abf4fe234fbdbdfad0c3c047fe6f3f14881bd81",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-gpg-keys-9.0-18.el9.noarch.rpm"],
)

rpm(
    name = "centos-stream-release-0__9.0-18.el9.x86_64",
    sha256 = "7078f8a58d6749b6d755cf375291c283318b5fc1b81ba550513e4570d77b961d",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-release-9.0-18.el9.noarch.rpm"],
)

rpm(
    name = "centos-stream-repos-0__9.0-18.el9.x86_64",
    sha256 = "447442d183d82e93a9516258dc272ba87207c9d5e755ca8e37d562dea92f1875",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/centos-stream-repos-9.0-18.el9.noarch.rpm"],
)

rpm(
    name = "checkpolicy-0__3.4-1.el9.x86_64",
    sha256 = "00030b411b38c1ed0babef38334c1ca7505d1548e616d8689eb2a724034d9b28",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/checkpolicy-3.4-1.el9.x86_64.rpm"],
)

rpm(
    name = "coreutils-single-0__8.32-33.el9.x86_64",
    sha256 = "ada0b3dfc46e2944206ca4af18a87067cc2a3d2f802ac7b49c627e3a46f1dd16",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/coreutils-single-8.32-33.el9.x86_64.rpm"],
)

rpm(
    name = "cracklib-0__2.9.6-27.el9.x86_64",
    sha256 = "be9deb2efd06b4b2c1c130acae94c687161d04830119e65a989d904ba9fd1864",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cracklib-2.9.6-27.el9.x86_64.rpm"],
)

rpm(
    name = "cracklib-dicts-0__2.9.6-27.el9.x86_64",
    sha256 = "01df2a72fcdf988132e82764ce1a22a5a9513fa253b54e17d23058bdb53c2d85",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cracklib-dicts-2.9.6-27.el9.x86_64.rpm"],
)

rpm(
    name = "crypto-policies-0__20221215-1.git9a18988.el9.x86_64",
    sha256 = "9a132069f88a63b7b2c146ffc4c17ed80f8ff57b69eb5affdec9cd1306dcf6ad",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/crypto-policies-20221215-1.git9a18988.el9.noarch.rpm"],
)

rpm(
    name = "cryptsetup-libs-0__2.6.0-2.el9.x86_64",
    sha256 = "3ebe050f05bbf3dcd2342240a7ce3013beb2af411e4eb1e68d31190f4aeae913",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cryptsetup-libs-2.6.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "curl-0__7.76.1-21.el9.x86_64",
    sha256 = "47129405ce7bdde445079794b228bc51a6e9609fa9d68f6f812682fbe0bb962b",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/curl-7.76.1-21.el9.x86_64.rpm"],
)

rpm(
    name = "cyrus-sasl-0__2.1.27-21.el9.x86_64",
    sha256 = "b919e98a1da12adaf63056e4b3fe068541fdcaea5b891ac32c50f70074e7a682",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cyrus-sasl-2.1.27-21.el9.x86_64.rpm"],
)

rpm(
    name = "cyrus-sasl-gssapi-0__2.1.27-21.el9.x86_64",
    sha256 = "c7cba5ec41adada2d95348705d91a5ef7b4bca2f82ca22440e881ad28d2d27d0",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cyrus-sasl-gssapi-2.1.27-21.el9.x86_64.rpm"],
)

rpm(
    name = "cyrus-sasl-lib-0__2.1.27-21.el9.x86_64",
    sha256 = "fd4292a29759f9531bbc876d1818e7a83ccac76907234002f598671d7b338469",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/cyrus-sasl-lib-2.1.27-21.el9.x86_64.rpm"],
)

rpm(
    name = "daxctl-libs-0__71.1-8.el9.x86_64",
    sha256 = "95bbf4ffb69cebc022fe3a2b35b828978d47e5b016747197ed5be34a57712432",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/daxctl-libs-71.1-8.el9.x86_64.rpm"],
)

rpm(
    name = "dbus-1__1.12.20-7.el9.x86_64",
    sha256 = "a1111141d56f30e206be37269294af8de24da02e65024187f9b4d474656b573a",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-1.12.20-7.el9.x86_64.rpm"],
)

rpm(
    name = "dbus-broker-0__28-7.el9.x86_64",
    sha256 = "dd65bddd728ed08dcdba5d06b5a5af9f958e5718e8cab938783241bd8f4d1131",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-broker-28-7.el9.x86_64.rpm"],
)

rpm(
    name = "dbus-common-1__1.12.20-7.el9.x86_64",
    sha256 = "b70a359af020f34116139d96e7f138c10e1bb32a219836b88045ffaa7f4a36a5",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-common-1.12.20-7.el9.noarch.rpm"],
)

rpm(
    name = "dbus-libs-1__1.12.20-7.el9.x86_64",
    sha256 = "c3d0a716e7b8e248a6662abbe7b34c46df8255b006dde1c98d29e1d18b0599e9",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dbus-libs-1.12.20-7.el9.x86_64.rpm"],
)

rpm(
    name = "device-mapper-9__1.02.187-3.el9.x86_64",
    sha256 = "87582eca70109e8d7d5845a645709e3a9489f0805721272852b25ff57f0cf92a",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/device-mapper-1.02.187-3.el9.x86_64.rpm"],
)

rpm(
    name = "device-mapper-libs-9__1.02.187-3.el9.x86_64",
    sha256 = "16d1725cb37a9aff6d6f9548b9d7583d5c6d77a4b489d22a53919888ff43b9ca",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/device-mapper-libs-1.02.187-3.el9.x86_64.rpm"],
)

rpm(
    name = "device-mapper-multipath-libs-0__0.8.7-16.el9.x86_64",
    sha256 = "7fd929a4a8a38f235dd1669a660ad7f982c9e483251c4794caf4bbca33a2b304",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/device-mapper-multipath-libs-0.8.7-16.el9.x86_64.rpm"],
)

rpm(
    name = "diffutils-0__3.7-12.el9.x86_64",
    sha256 = "fdebefc46badf2e700e00582041a0e5f5183dd4fdc04badfe47c91f030cea0ce",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/diffutils-3.7-12.el9.x86_64.rpm"],
)

rpm(
    name = "dmidecode-1__3.3-7.el9.x86_64",
    sha256 = "2afb32bf0c30908817d57d221dbded83917aa8a88d2586e98ce548bad4f86e3d",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/dmidecode-3.3-7.el9.x86_64.rpm"],
)

rpm(
    name = "dwz-0__0.14-3.el9.x86_64",
    sha256 = "781c9a7a041882cc0f766bef8027babab01d520359a9d5bc4fa18053dbaee25c",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/dwz-0.14-3.el9.x86_64.rpm"],
)

rpm(
    name = "e2fsprogs-libs-0__1.46.5-3.el9.x86_64",
    sha256 = "0626ca08ef0d4ddafbb7679eb3915c61f0496038f92263529715681952854d20",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/e2fsprogs-libs-1.46.5-3.el9.x86_64.rpm"],
)

rpm(
    name = "edk2-ovmf-0__20221207gitfff6d81270b5-1.el9.x86_64",
    sha256 = "09400c06968dfd088a97c76b4460ac6cf1d86699f29287105385c4497b675213",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/edk2-ovmf-20221207gitfff6d81270b5-1.el9.noarch.rpm"],
)

rpm(
    name = "efi-srpm-macros-0__4-9.el9.x86_64",
    sha256 = "f406e81e8036226e8bf7d0c22a50632f6c8ea7cc3cfe552c5d00917c2879fc62",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/efi-srpm-macros-4-9.el9.noarch.rpm"],
)

rpm(
    name = "elfutils-libelf-0__0.188-3.el9.x86_64",
    sha256 = "0991cf08a00a872558c419f9c0fb8011d34c205c573e12d1afd388055a051530",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/elfutils-libelf-0.188-3.el9.x86_64.rpm"],
)

rpm(
    name = "expat-0__2.5.0-1.el9.x86_64",
    sha256 = "b5092845377c3505cd072a896c443abe5da21d3c6c6cb23d917db159905178a6",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/expat-2.5.0-1.el9.x86_64.rpm"],
)

rpm(
    name = "file-0__5.39-10.el9.x86_64",
    sha256 = "5127d8fba1f3b07e2982a4f21a2e4fa0f7dfb089681b2f10e267f5908735d625",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/file-5.39-10.el9.x86_64.rpm"],
)

rpm(
    name = "file-libs-0__5.39-10.el9.x86_64",
    sha256 = "da4dcbcf8f49bc84db988884a208f823cf1876fa5db79a05a66f5f0a30f67a01",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/file-libs-5.39-10.el9.x86_64.rpm"],
)

rpm(
    name = "filesystem-0__3.16-2.el9.x86_64",
    sha256 = "b69a472751268a1b9acd566dc7aa486fc1d6c8cb6d23f36d6a6dfead62e71475",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/filesystem-3.16-2.el9.x86_64.rpm"],
)

rpm(
    name = "findutils-1__4.8.0-5.el9.x86_64",
    sha256 = "552548e6d6f9623ccd9d31bb185bba3a66730da6e9d02296b417d501356c3848",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/findutils-4.8.0-5.el9.x86_64.rpm"],
)

rpm(
    name = "flac-libs-0__1.3.3-10.el9.x86_64",
    sha256 = "9348f074f4689b52f5217105117032fa95ec0406777519bc87666af6492a4d80",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/flac-libs-1.3.3-10.el9.x86_64.rpm"],
)

rpm(
    name = "fonts-filesystem-1__2.0.5-7.el9.1.x86_64",
    sha256 = "c79fa96aa7fb447975497dd50c94002ee73d01171343f8ee14032d06adb58a92",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/fonts-filesystem-2.0.5-7.el9.1.noarch.rpm"],
)

rpm(
    name = "fonts-srpm-macros-1__2.0.5-7.el9.1.x86_64",
    sha256 = "94a286ad228c795a359c4f672565fe852033fa40d3f2da3c9c24d1d3aac9e61c",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/fonts-srpm-macros-2.0.5-7.el9.1.noarch.rpm"],
)

rpm(
    name = "fuse-0__2.9.9-15.el9.x86_64",
    sha256 = "f0f8b58029ffddf73c5147c67c8e5f90f60e0e315f195c25695ceb0e9fec9d4b",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/fuse-2.9.9-15.el9.x86_64.rpm"],
)

rpm(
    name = "fuse-common-0__3.10.2-5.el9.x86_64",
    sha256 = "a156d82484b61b6323524631d80c8d184042e8819a6d86a1b9b3076f3b5f3612",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/fuse-common-3.10.2-5.el9.x86_64.rpm"],
)

rpm(
    name = "fuse-libs-0__2.9.9-15.el9.x86_64",
    sha256 = "610c601daea8fa587c3ee43f2af06c25c506caf4588bf214e04de7eb960b95fa",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/fuse-libs-2.9.9-15.el9.x86_64.rpm"],
)

rpm(
    name = "gawk-0__5.1.0-6.el9.x86_64",
    sha256 = "6e6d77b76b1e89fe6f012cdc16111bea35eb4ceedac5040e5d81b5a066429af8",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gawk-5.1.0-6.el9.x86_64.rpm"],
)

rpm(
    name = "gdbm-libs-1__1.19-4.el9.x86_64",
    sha256 = "8cd5a78cab8783dd241c52c4fcda28fb111c443887dd6d0fe38385e8383c98b3",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gdbm-libs-1.19-4.el9.x86_64.rpm"],
)

rpm(
    name = "gettext-0__0.21-7.el9.x86_64",
    sha256 = "386905ddacb2614d519ec5dbaf038d40dbc44307b0edaa0bd3e6a5baa405a7b8",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gettext-0.21-7.el9.x86_64.rpm"],
)

rpm(
    name = "gettext-libs-0__0.21-7.el9.x86_64",
    sha256 = "1388fca61334c67cac638edba2459b362cc401c8ff5ab8d7d5ca387b0ffc8786",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gettext-libs-0.21-7.el9.x86_64.rpm"],
)

rpm(
    name = "ghc-srpm-macros-0__1.5.0-6.el9.x86_64",
    sha256 = "42664f2bf33095f42ea7b4b104b6e0556fa1f9db815ccc10e55c913ed61f1097",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/ghc-srpm-macros-1.5.0-6.el9.noarch.rpm"],
)

rpm(
    name = "glib-networking-0__2.68.3-3.el9.x86_64",
    sha256 = "ea106ccc142daf5016626cfe5c4f0a2d97e700ae7ad4780835e899897b63317f",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glib-networking-2.68.3-3.el9.x86_64.rpm"],
)

rpm(
    name = "glib2-0__2.68.4-6.el9.x86_64",
    sha256 = "e8f2fd55fa576c57811f260ff5416411d39013bfe4b74bc8db87b8fc49b82705",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glib2-2.68.4-6.el9.x86_64.rpm"],
)

rpm(
    name = "glibc-0__2.34-54.el9.x86_64",
    sha256 = "b75a4b8cb5bba399ca6b0e85fcdb51437cf2e4d478662d7fe94a55aee7afe6a2",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-2.34-54.el9.x86_64.rpm"],
)

rpm(
    name = "glibc-common-0__2.34-54.el9.x86_64",
    sha256 = "d88a5243ac2e1039436ea6d84600b8bd69258bc815aa8c560e9c9fc5edd09316",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-common-2.34-54.el9.x86_64.rpm"],
)

rpm(
    name = "glibc-langpack-cy-0__2.34-54.el9.x86_64",
    sha256 = "04cb3da82c9e3348c7e2977ce2409399ccdc019272f33682921e920f6e81e454",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-langpack-cy-2.34-54.el9.x86_64.rpm"],
)

rpm(
    name = "glibc-langpack-fr-0__2.34-54.el9.x86_64",
    sha256 = "f05670df408197cc44f34d1ca395cf2a947221449ef5f8a3c7b122a0fb085b47",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/glibc-langpack-fr-2.34-54.el9.x86_64.rpm"],
)

rpm(
    name = "gmp-1__6.2.0-10.el9.x86_64",
    sha256 = "1a6ededc80029ef258288ddbf24bcce7c6228647841416950c88e3f14b7258a2",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gmp-6.2.0-10.el9.x86_64.rpm"],
)

rpm(
    name = "gnupg2-0__2.3.3-2.el9.x86_64",
    sha256 = "d537e48c6947c6086d1af21b81b2619931b0ff708606d7545e388bbea05dcf32",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gnupg2-2.3.3-2.el9.x86_64.rpm"],
)

rpm(
    name = "gnutls-0__3.7.6-15.el9.x86_64",
    sha256 = "ebde09c2897410a30390cae990151984020de4ff24269ab3fef770ef6207c9ad",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gnutls-3.7.6-15.el9.x86_64.rpm"],
)

rpm(
    name = "gnutls-dane-0__3.7.6-15.el9.x86_64",
    sha256 = "5b20a5b19a552809500b1cf97f743906c867c8ee6b8d39e0f4d986ae816a26cb",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/gnutls-dane-3.7.6-15.el9.x86_64.rpm"],
)

rpm(
    name = "gnutls-utils-0__3.7.6-15.el9.x86_64",
    sha256 = "19978aeba32743e002d47f85c68eba3141351ad048a918e2227b2db286d0280d",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/gnutls-utils-3.7.6-15.el9.x86_64.rpm"],
)

rpm(
    name = "go-srpm-macros-0__3.0.9-9.el9.x86_64",
    sha256 = "d2e036ba4738531bc26e8f6bd1c11b15fd1e136b79e961cf4b60ef5b8ea366a6",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/go-srpm-macros-3.0.9-9.el9.noarch.rpm"],
)

rpm(
    name = "grep-0__3.6-5.el9.x86_64",
    sha256 = "10a41b66b1fbd6eb055178e22c37199e5b49b4852e77c806f7af7211044a4a55",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/grep-3.6-5.el9.x86_64.rpm"],
)

rpm(
    name = "groff-base-0__1.22.4-10.el9.x86_64",
    sha256 = "f8f02725766bef0fdf3db124d7862848e692518ce04919fb1a583f013bbbabfb",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/groff-base-1.22.4-10.el9.x86_64.rpm"],
)

rpm(
    name = "gsettings-desktop-schemas-0__40.0-6.el9.x86_64",
    sha256 = "9935991dc0dfb2eda15db01d388d4a018ee3aaf0c5f8ffa4ca1297f05d62db33",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gsettings-desktop-schemas-40.0-6.el9.x86_64.rpm"],
)

rpm(
    name = "gsm-0__1.0.19-6.el9.x86_64",
    sha256 = "d4c242d54a503c80c07467b68e212986fdb65e4afb8487150143b4490b05177c",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/gsm-1.0.19-6.el9.x86_64.rpm"],
)

rpm(
    name = "gssproxy-0__0.8.4-4.el9.x86_64",
    sha256 = "bc7b37a4bc3342ca7884f0166b4124d68b51b75ead9f8e996ddbd0125ab571d5",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gssproxy-0.8.4-4.el9.x86_64.rpm"],
)

rpm(
    name = "guestfs-tools-0__1.48.2-8.el9.x86_64",
    sha256 = "b423ad40665e919d487278ae7d5c88734c724c3dabd14b3201bb44b6e11554b1",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/guestfs-tools-1.48.2-8.el9.x86_64.rpm"],
)

rpm(
    name = "gzip-0__1.12-1.el9.x86_64",
    sha256 = "e8d7783c666a58ab870246b04eb0ea22965123fe284697d2c0e1e6dbf10ea861",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/gzip-1.12-1.el9.x86_64.rpm"],
)

rpm(
    name = "hexedit-0__1.6-1.el9.x86_64",
    sha256 = "8c0781f044f9e45329cfc0f4c7d7acd65c9f779b34816c205279f977919e856f",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/hexedit-1.6-1.el9.x86_64.rpm"],
)

rpm(
    name = "hivex-libs-0__1.3.21-3.el9.x86_64",
    sha256 = "3b0b567737f8a78e9264a07f935b25098f505d2b46653dba944919da85020ef7",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/hivex-libs-1.3.21-3.el9.x86_64.rpm"],
)

rpm(
    name = "hwdata-0__0.348-9.6.el9.x86_64",
    sha256 = "fc3170e828586bfa910bae61086ed61418df2ccac4006f21372d6832caa2767d",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/hwdata-0.348-9.6.el9.noarch.rpm"],
)

rpm(
    name = "inih-0__49-6.el9.x86_64",
    sha256 = "47222549ca25e54991194feece4ef95333a3e29ace0bb3fc45bb6a60887347c2",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/inih-49-6.el9.x86_64.rpm"],
)

rpm(
    name = "iproute-0__5.18.0-1.el9.x86_64",
    sha256 = "7396e9caf6a3b98de2fe82bad2d8b3607c1e0abf1dcbd9c86c8fb378d605e92a",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/iproute-5.18.0-1.el9.x86_64.rpm"],
)

rpm(
    name = "iproute-tc-0__5.18.0-1.el9.x86_64",
    sha256 = "de2e6ab190d0515cd0190c8341379efc7fdf631d464bddac1227ab45efa4693b",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/iproute-tc-5.18.0-1.el9.x86_64.rpm"],
)

rpm(
    name = "iptables-libs-0__1.8.8-6.el9.x86_64",
    sha256 = "c1e4ebce15d824604e777993f46b94706239044c81bc5240e9541b1ae93485a5",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/iptables-libs-1.8.8-6.el9.x86_64.rpm"],
)

rpm(
    name = "ipxe-roms-qemu-0__20200823-9.git4bd064de.el9.x86_64",
    sha256 = "fa304f6cffa4a84a8aae1e0d2dd10606ffb51b88d9568b7da92ffd63acb14851",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/ipxe-roms-qemu-20200823-9.git4bd064de.el9.noarch.rpm"],
)

rpm(
    name = "jansson-0__2.14-1.el9.x86_64",
    sha256 = "c3fb9f8020f978f9b392709996e62e4ddb6cb19074635af3338487195b688f66",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/jansson-2.14-1.el9.x86_64.rpm"],
)

rpm(
    name = "json-c-0__0.14-11.el9.x86_64",
    sha256 = "1a75404c6bc8c1369914077dc99480e73bf13a40f15fd1cd8afc792b8600adf8",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/json-c-0.14-11.el9.x86_64.rpm"],
)

rpm(
    name = "json-glib-0__1.6.6-1.el9.x86_64",
    sha256 = "d850cb45d31fe84cb50cb1fa26eb5418633aae1f0dcab8b7ebadd3bd3e340956",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/json-glib-1.6.6-1.el9.x86_64.rpm"],
)

rpm(
    name = "kernel-srpm-macros-0__1.0-11.el9.x86_64",
    sha256 = "95be09143def547bcaa9ac90ab5a3e0aae72513482b838e279a2f72d729f4d94",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/kernel-srpm-macros-1.0-11.el9.noarch.rpm"],
)

rpm(
    name = "keyutils-0__1.6.3-1.el9.x86_64",
    sha256 = "bc9b6262006e7722b7936e3d1e5079d7281f96e161bcd0aa93328564a32984bb",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/keyutils-1.6.3-1.el9.x86_64.rpm"],
)

rpm(
    name = "keyutils-libs-0__1.6.3-1.el9.x86_64",
    sha256 = "aef982501694486a27411c68698886d76ec70c5cd10bfe619501e7e4c36f50a9",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/keyutils-libs-1.6.3-1.el9.x86_64.rpm"],
)

rpm(
    name = "kmod-0__28-7.el9.x86_64",
    sha256 = "3d4bc7935959a109a10020d0d19a5e059719ae4c99c5f32d3020ff6da47d53ea",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/kmod-28-7.el9.x86_64.rpm"],
)

rpm(
    name = "kmod-libs-0__28-7.el9.x86_64",
    sha256 = "0727ff3131223446158aaec88cbf8f894a9e3592e73f231a1802629518eeb64b",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/kmod-libs-28-7.el9.x86_64.rpm"],
)

rpm(
    name = "krb5-libs-0__1.19.1-22.el9.x86_64",
    sha256 = "81195fcb28dca19447c75d1eff6b62c0b4f849e6b492c992e890bb65aca55734",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/krb5-libs-1.19.1-22.el9.x86_64.rpm"],
)

rpm(
    name = "less-0__590-1.el9.x86_64",
    sha256 = "75ec2628be5ebe149a79b00f2160b9653297651d5ea291022e053719d2ff07f5",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/less-590-1.el9.x86_64.rpm"],
)

rpm(
    name = "libX11-0__1.7.0-7.el9.x86_64",
    sha256 = "053eedbf427210b3cf0aacfbd04c56b762034b0fd71fe47e01117b90e7128faa",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libX11-1.7.0-7.el9.x86_64.rpm"],
)

rpm(
    name = "libX11-common-0__1.7.0-7.el9.x86_64",
    sha256 = "72c5b03d30e572ceb1635a539eee3ec48ad41c5ee1132d4a64e0cc86f0933990",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libX11-common-1.7.0-7.el9.noarch.rpm"],
)

rpm(
    name = "libX11-xcb-0__1.7.0-7.el9.x86_64",
    sha256 = "6d9196e9683706525048db8ae39583ab56ec4f45364717528d6c63ebc8451804",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libX11-xcb-1.7.0-7.el9.x86_64.rpm"],
)

rpm(
    name = "libXau-0__1.0.9-8.el9.x86_64",
    sha256 = "a0c14be959891eaff9097c1ec4d7c4b044301623d4080585cee72d740cd659ae",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libXau-1.0.9-8.el9.x86_64.rpm"],
)

rpm(
    name = "libXext-0__1.3.4-8.el9.x86_64",
    sha256 = "3714ed495275ffee5a8d374ae401cdef2c7bd30d2aebf90aecf4f1be8d6f896d",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libXext-1.3.4-8.el9.x86_64.rpm"],
)

rpm(
    name = "libXfixes-0__5.0.3-16.el9.x86_64",
    sha256 = "309d12ca62069d02b6cf9d96d8d97de6b0267134ffd4b6952adc561269c8c9ca",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libXfixes-5.0.3-16.el9.x86_64.rpm"],
)

rpm(
    name = "libXxf86vm-0__1.1.4-18.el9.x86_64",
    sha256 = "fe95e780bab5c4dda66acefaf35bdc6dddd483928cd2472cffa89a67ffc6b53b",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libXxf86vm-1.1.4-18.el9.x86_64.rpm"],
)

rpm(
    name = "libacl-0__2.3.1-3.el9.x86_64",
    sha256 = "fd829e9a03f6d321313002d6fcb37ee0434f548aa75fcd3ecdbdd891115de6a7",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libacl-2.3.1-3.el9.x86_64.rpm"],
)

rpm(
    name = "libaio-0__0.3.111-13.el9.x86_64",
    sha256 = "7d9d4d37e86ba94bb941e2dad40c90a157aaa0602f02f3f90e76086515f439be",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libaio-0.3.111-13.el9.x86_64.rpm"],
)

rpm(
    name = "libarchive-0__3.5.3-4.el9.x86_64",
    sha256 = "4c53176eafd8c449aef704b8fbc2d5401bb7d2ea0a67961956f318f2e9a2c7a4",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libarchive-3.5.3-4.el9.x86_64.rpm"],
)

rpm(
    name = "libassuan-0__2.5.5-3.el9.x86_64",
    sha256 = "3f7ab80145768029619033b31406a9aeef8c8f0d42a0c94ad464d8a3405e12b0",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libassuan-2.5.5-3.el9.x86_64.rpm"],
)

rpm(
    name = "libasyncns-0__0.8-22.el9.x86_64",
    sha256 = "5214799bb68b6933ec92c4b183777565c1949757f545b6d84e13da189d91cb86",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libasyncns-0.8-22.el9.x86_64.rpm"],
)

rpm(
    name = "libattr-0__2.5.1-3.el9.x86_64",
    sha256 = "d4db095a015e84065f27a642ee7829cd1690041ba8c51501f908cc34760c9409",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libattr-2.5.1-3.el9.x86_64.rpm"],
)

rpm(
    name = "libbasicobjects-0__0.1.1-53.el9.x86_64",
    sha256 = "14ce3dd811d88dddc4009c12094cd0e52bbcabe0f2463bdfcc4124c620fb13d5",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libbasicobjects-0.1.1-53.el9.x86_64.rpm"],
)

rpm(
    name = "libblkid-0__2.37.4-9.el9.x86_64",
    sha256 = "cb09fe87839c17ae2726459d4d5f3e2a7396071b03cda70201a6d1e9db5e7504",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libblkid-2.37.4-9.el9.x86_64.rpm"],
)

rpm(
    name = "libbpf-2__0.8.0-1.el9.x86_64",
    sha256 = "bcdb39bb08642fcda7ef35f9d0578830428dd283b2723aef1b949caa853c1c80",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libbpf-0.8.0-1.el9.x86_64.rpm"],
)

rpm(
    name = "libbrotli-0__1.0.9-6.el9.x86_64",
    sha256 = "10b93bc07c62f31b96cbd4141a645880e76a2bc7d7163306ce2cc61a49616202",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libbrotli-1.0.9-6.el9.x86_64.rpm"],
)

rpm(
    name = "libcap-0__2.48-8.el9.x86_64",
    sha256 = "c41f91075ee8ca480c2631a485bcc74876b9317b4dc9bd66566da32313621bd7",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcap-2.48-8.el9.x86_64.rpm"],
)

rpm(
    name = "libcap-ng-0__0.8.2-7.el9.x86_64",
    sha256 = "62429b788acfb40dbc9da9951690c11e907e230879c790d139f73d0e85dd76f4",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcap-ng-0.8.2-7.el9.x86_64.rpm"],
)

rpm(
    name = "libcbor-0__0.7.0-5.el9.x86_64",
    sha256 = "ecbb61df93e6816276712d02a3013c591a8b58a8ef50ece98d814564565980ab",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcbor-0.7.0-5.el9.x86_64.rpm"],
)

rpm(
    name = "libcollection-0__0.7.0-53.el9.x86_64",
    sha256 = "07c24fc00d1fd088a7f2b16b6cf70b781aed6ed682f11c4bce3ab76cf56707fd",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcollection-0.7.0-53.el9.x86_64.rpm"],
)

rpm(
    name = "libcom_err-0__1.46.5-3.el9.x86_64",
    sha256 = "ef9db384c8fbfc0b8676aec1896070dc308cfc0c7b515ebbe556e0fea68318d0",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcom_err-1.46.5-3.el9.x86_64.rpm"],
)

rpm(
    name = "libconfig-0__1.7.2-9.el9.x86_64",
    sha256 = "e0d4d2cf8215404750c3975a19e2b7cd2c9e9e1e5c539d3fd93532775fd2ed16",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libconfig-1.7.2-9.el9.x86_64.rpm"],
)

rpm(
    name = "libcurl-0__7.76.1-21.el9.x86_64",
    sha256 = "fc334f46902adf5cf8e61aa63a36832e584cba9780c3efff520ac1d209f5c4d0",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcurl-7.76.1-21.el9.x86_64.rpm"],
)

rpm(
    name = "libcurl-minimal-0__7.76.1-21.el9.x86_64",
    sha256 = "7c1569f49951997f530469484552684efdacd8eec727a24ef3d768590880011f",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libcurl-minimal-7.76.1-21.el9.x86_64.rpm"],
)

rpm(
    name = "libdb-0__5.3.28-53.el9.x86_64",
    sha256 = "3a44d15d695944bde4e7290800b815f98bfd9cd6f6f868cec3e8991606f556d5",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libdb-5.3.28-53.el9.x86_64.rpm"],
)

rpm(
    name = "libdrm-0__2.4.114-1.el9.x86_64",
    sha256 = "0b2c7c122aec3d0eb693cdbdef69ba31a2fa7566bb349d9d70b5798cdc15e3c0",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libdrm-2.4.114-1.el9.x86_64.rpm"],
)

rpm(
    name = "libeconf-0__0.4.1-2.el9.x86_64",
    sha256 = "1d6fe169e74daff38ad5b0d6424c4d1b14545d5974c39e4421d20838a68f5892",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libeconf-0.4.1-2.el9.x86_64.rpm"],
)

rpm(
    name = "libedit-0__3.1-37.20210216cvs.el9.x86_64",
    sha256 = "61d7cbecce6847d13b142460c468cda0e825562987ed23b5dfe1eb1f3418e8bd",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libedit-3.1-37.20210216cvs.el9.x86_64.rpm"],
)

rpm(
    name = "libepoxy-0__1.5.5-4.el9.x86_64",
    sha256 = "7f282efb4675e8e2ffe4a8c75e737d2450c5273df60ebc311d388192d1353fbb",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libepoxy-1.5.5-4.el9.x86_64.rpm"],
)

rpm(
    name = "libev-0__4.33-5.el9.x86_64",
    sha256 = "9ee87c7d34e341bc7b136125ef5f1429a0b5fadaffcf888ab896b2c62c2b4e8d",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libev-4.33-5.el9.x86_64.rpm"],
)

rpm(
    name = "libevent-0__2.1.12-6.el9.x86_64",
    sha256 = "82179f6f214ddf523e143c16c3474ccf8832551c6305faf89edfbd83b3424d48",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libevent-2.1.12-6.el9.x86_64.rpm"],
)

rpm(
    name = "libfdisk-0__2.37.4-9.el9.x86_64",
    sha256 = "1e75c0e916ce41ca3fc04322f414aa295ccc2cb4ed9cc4f512d656f8726230ab",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libfdisk-2.37.4-9.el9.x86_64.rpm"],
)

rpm(
    name = "libfdt-0__1.6.0-7.el9.x86_64",
    sha256 = "a071b9d517505a2ff8642de7ac094faa689b96122c0a3e9ce86933aa1dea525f",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libfdt-1.6.0-7.el9.x86_64.rpm"],
)

rpm(
    name = "libffi-0__3.4.2-7.el9.x86_64",
    sha256 = "f0ac4b6454d4018833dd10e3f437d8271c7c6a628d99b37e75b83af890b86bc4",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libffi-3.4.2-7.el9.x86_64.rpm"],
)

rpm(
    name = "libfido2-0__1.6.0-7.el9.x86_64",
    sha256 = "aa274e6bcc5bff09db5f157eafdcff8308c20b74c2103fa3eb07cb71ee937110",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libfido2-1.6.0-7.el9.x86_64.rpm"],
)

rpm(
    name = "libgcc-0__11.3.1-4.3.el9.x86_64",
    sha256 = "07cf39318f2cceb424b40f6fe6441ecd25989fb4a7f0403357484db9d00481ea",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgcc-11.3.1-4.3.el9.x86_64.rpm"],
)

rpm(
    name = "libgcrypt-0__1.10.0-8.el9.x86_64",
    sha256 = "b43f2b01eaa2c2226a7c371f87d0a98f3c5291519f4e9e9e4a6f53320544e7fe",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgcrypt-1.10.0-8.el9.x86_64.rpm"],
)

rpm(
    name = "libglvnd-1__1.3.4-1.el9.x86_64",
    sha256 = "129af138450cf5aba44d7a72bcccbf2c691dec72a29ac2bf769ae7dec2092ee3",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libglvnd-1.3.4-1.el9.x86_64.rpm"],
)

rpm(
    name = "libglvnd-egl-1__1.3.4-1.el9.x86_64",
    sha256 = "6c815bac0524572490c81cfe5331b55fb3c87aef99ff96a5028210bc2397497b",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libglvnd-egl-1.3.4-1.el9.x86_64.rpm"],
)

rpm(
    name = "libglvnd-glx-1__1.3.4-1.el9.x86_64",
    sha256 = "9cf25dfa3a3fb57f2142f7941b7be1c020e214e01863ceb3901cfa2204b558e0",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libglvnd-glx-1.3.4-1.el9.x86_64.rpm"],
)

rpm(
    name = "libgomp-0__11.3.1-4.3.el9.x86_64",
    sha256 = "0ab22498aa557f01b73f58ddc65039bcdb33a9259b5f049842d3eb87c294b345",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgomp-11.3.1-4.3.el9.x86_64.rpm"],
)

rpm(
    name = "libgpg-error-0__1.42-5.el9.x86_64",
    sha256 = "a1883804c376f737109f4dff06077d1912b90150a732d11be7bc5b3b67e512fe",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libgpg-error-1.42-5.el9.x86_64.rpm"],
)

rpm(
    name = "libguestfs-1__1.48.4-4.el9.x86_64",
    sha256 = "ab50fdd378fb6fad46a61cb6d33ac582b7d5d9e5c4fcf3e8dfd55e8a630fd05f",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libguestfs-1.48.4-4.el9.x86_64.rpm"],
)

rpm(
    name = "libguestfs-winsupport-0__9.2-1.el9.x86_64",
    sha256 = "5693334f1c9b64834efcc7ae31d0dbb79ccf90847c820312d54fd89ec504c44a",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libguestfs-winsupport-9.2-1.el9.x86_64.rpm"],
)

rpm(
    name = "libguestfs-xfs-1__1.48.4-4.el9.x86_64",
    sha256 = "03b4872e050036ece6113f730f997424d57800b5d6b313501ea0fb6f924a2d16",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libguestfs-xfs-1.48.4-4.el9.x86_64.rpm"],
)

rpm(
    name = "libibverbs-0__41.0-3.el9.x86_64",
    sha256 = "b7b3673aa94b178533d0934bf9b30cc28caa071646add99171baef5a5882d92a",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libibverbs-41.0-3.el9.x86_64.rpm"],
)

rpm(
    name = "libidn2-0__2.3.0-7.el9.x86_64",
    sha256 = "f7fa1ad2fcd86beea5d4d965994c21dc98f47871faff14f73940190c754ab244",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libidn2-2.3.0-7.el9.x86_64.rpm"],
)

rpm(
    name = "libini_config-0__1.3.1-53.el9.x86_64",
    sha256 = "fb7dbaeb7c172663cab3029c4efaf80230bcba4abf1604cc6cc00993b5d9659e",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libini_config-1.3.1-53.el9.x86_64.rpm"],
)

rpm(
    name = "libksba-0__1.5.1-5.el9.x86_64",
    sha256 = "e3eff03293ecda05f77473152b5f1cc5b64072b8578f9c5dcb2597c137427b4f",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libksba-1.5.1-5.el9.x86_64.rpm"],
)

rpm(
    name = "libmnl-0__1.0.4-15.el9.x86_64",
    sha256 = "a70fdda85cd771ef5bf5b17c2996e4ff4d21c2e5b1eece1764a87f12e720ab68",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libmnl-1.0.4-15.el9.x86_64.rpm"],
)

rpm(
    name = "libmount-0__2.37.4-9.el9.x86_64",
    sha256 = "10fefd21b2d0e3b4c48e87fc29303eb493589e68d4b5edccd43ced8154904874",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libmount-2.37.4-9.el9.x86_64.rpm"],
)

rpm(
    name = "libnbd-0__1.12.6-1.el9.x86_64",
    sha256 = "048064607c3ad717f192cdaaffa5bcacad6dbc3621df1bed9642259486fcbd54",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libnbd-1.12.6-1.el9.x86_64.rpm"],
)

rpm(
    name = "libnetfilter_conntrack-0__1.0.9-1.el9.x86_64",
    sha256 = "f81a0188964268ae9e1d53d99dba3ef96a65fe2fb00bc8fe6c39cedfdd364f44",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnetfilter_conntrack-1.0.9-1.el9.x86_64.rpm"],
)

rpm(
    name = "libnfnetlink-0__1.0.1-21.el9.x86_64",
    sha256 = "64f54f412cc0ee6fe82be7557f471a06f6bf1f5bba1d6fe0ad1879e5a62d7c95",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnfnetlink-1.0.1-21.el9.x86_64.rpm"],
)

rpm(
    name = "libnfsidmap-1__2.5.4-17.el9.x86_64",
    sha256 = "228b1cb4f1d8f8be6951ed39db6474cbfdba91c1afa4d09449f5dca42ef106f7",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnfsidmap-2.5.4-17.el9.x86_64.rpm"],
)

rpm(
    name = "libnghttp2-0__1.43.0-5.el9.x86_64",
    sha256 = "58c5d589ee370951b98e908ac05a5a6154d52dbb8cf2067583ccdd10cdf099bf",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnghttp2-1.43.0-5.el9.x86_64.rpm"],
)

rpm(
    name = "libnl3-0__3.7.0-1.el9.x86_64",
    sha256 = "8abf9bf3f62df66aeed157fc9f9494a2ea792eb11eb221caa17ce7f97330a2f3",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libnl3-3.7.0-1.el9.x86_64.rpm"],
)

rpm(
    name = "libogg-2__1.3.4-6.el9.x86_64",
    sha256 = "ff8b1d6cf009aef8c8d1d5508c456479f62b7069e1d6a3f225b6233f645c82ce",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libogg-1.3.4-6.el9.x86_64.rpm"],
)

rpm(
    name = "libosinfo-0__1.10.0-1.el9.x86_64",
    sha256 = "ace3a92175ee1be1f5c3a1d31bd702c49076eea7f4d6e859fc301832424d3dc9",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libosinfo-1.10.0-1.el9.x86_64.rpm"],
)

rpm(
    name = "libpath_utils-0__0.2.1-53.el9.x86_64",
    sha256 = "0a2519647ef22df7c975fa2851da713e67361ff33f2bff05f91cb588b2722772",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libpath_utils-0.2.1-53.el9.x86_64.rpm"],
)

rpm(
    name = "libpcap-14__1.10.0-4.el9.x86_64",
    sha256 = "c76c9887f6b9d218300b24f1adee1b0d9104d25152df3fcd005002d12e12399e",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libpcap-1.10.0-4.el9.x86_64.rpm"],
)

rpm(
    name = "libpciaccess-0__0.16-6.el9.x86_64",
    sha256 = "c07ac2537076fc2b772f5a0dd2852b3c61aa0b7502b2ff01fcdaa02329841d87",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libpciaccess-0.16-6.el9.x86_64.rpm"],
)

rpm(
    name = "libpipeline-0__1.5.3-4.el9.x86_64",
    sha256 = "155993a46a21cd613b856f7daef85b74889fda0bbd653d8f93bde5b34324fea4",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libpipeline-1.5.3-4.el9.x86_64.rpm"],
)

rpm(
    name = "libpkgconf-0__1.7.3-10.el9.x86_64",
    sha256 = "2dc8b201f4e24ca65fe6389fec8901eb84d48519cc44a6b0e474d7859370f389",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libpkgconf-1.7.3-10.el9.x86_64.rpm"],
)

rpm(
    name = "libpmem-0__1.12.1-1.el9.x86_64",
    sha256 = "5377dcb3b4ca48eb056a998d3a684eb68e8d059e2a26844cda8535d8f125fc83",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libpmem-1.12.1-1.el9.x86_64.rpm"],
)

rpm(
    name = "libpng-2__1.6.37-12.el9.x86_64",
    sha256 = "b3f3a689918dc50a9bc41c33abf1a36bdb8e4a707daac77a91e0814407b07ae3",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libpng-1.6.37-12.el9.x86_64.rpm"],
)

rpm(
    name = "libproxy-0__0.4.15-35.el9.x86_64",
    sha256 = "0042c2dd5a88f7f1db096426bb1f6557e7d790eabca01a086afd832e47217ee1",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libproxy-0.4.15-35.el9.x86_64.rpm"],
)

rpm(
    name = "libpsl-0__0.21.1-5.el9.x86_64",
    sha256 = "42bd5fb4b34c993c103ea2d47fc69a0fcc231fcfb88646ed55403519868caa94",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libpsl-0.21.1-5.el9.x86_64.rpm"],
)

rpm(
    name = "libpwquality-0__1.4.4-8.el9.x86_64",
    sha256 = "93f00e5efac1e3f1ecbc0d6a4c068772cb12912cd20c9ea58716d6c0cd004886",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libpwquality-1.4.4-8.el9.x86_64.rpm"],
)

rpm(
    name = "librados2-2__16.2.4-5.el9.x86_64",
    sha256 = "6786852b684ea584343d560c7e0a7303790f1129d320b493df3c45a73850d073",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/librados2-16.2.4-5.el9.x86_64.rpm"],
)

rpm(
    name = "librbd1-2__16.2.4-5.el9.x86_64",
    sha256 = "d4d1549eb600af4efc546630fc89170905e8b2c174d8528315d36451879eacc9",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/librbd1-16.2.4-5.el9.x86_64.rpm"],
)

rpm(
    name = "librdmacm-0__41.0-3.el9.x86_64",
    sha256 = "62661a80fc924f55f81a0746cd428668e3d00103550c9d67aca953b5eb9eb33f",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/librdmacm-41.0-3.el9.x86_64.rpm"],
)

rpm(
    name = "libref_array-0__0.1.5-53.el9.x86_64",
    sha256 = "7a7eaf030a25e866148daa6b38ac6f49afeba63b66f11040cc7b5b5522977d1e",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libref_array-0.1.5-53.el9.x86_64.rpm"],
)

rpm(
    name = "libseccomp-0__2.5.2-2.el9.x86_64",
    sha256 = "d5c1c4473ebf5fd9c605eb866118d7428cdec9b188db18e45545801cc2a689c3",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libseccomp-2.5.2-2.el9.x86_64.rpm"],
)

rpm(
    name = "libselinux-0__3.4-3.el9.x86_64",
    sha256 = "9be03d8382bf156d9cda703e453d213bde9f53389ec6841fb4cb900f13e22d99",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libselinux-3.4-3.el9.x86_64.rpm"],
)

rpm(
    name = "libselinux-utils-0__3.4-3.el9.x86_64",
    sha256 = "fde4963b3512e33efd007a47f4adf893e5bd11b9a6fc4d41c329c67a98132204",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libselinux-utils-3.4-3.el9.x86_64.rpm"],
)

rpm(
    name = "libsemanage-0__3.4-2.el9.x86_64",
    sha256 = "f2a78bfe03b84b3722e5b0f17cb8b21e5b258e4221b3c0130dcd3e6ed00f43b7",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsemanage-3.4-2.el9.x86_64.rpm"],
)

rpm(
    name = "libsepol-0__3.4-3.el9.x86_64",
    sha256 = "9547873cf4e7b6089645849f514b8651e9adf3f528311add65cc0495777876c0",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsepol-3.4-3.el9.x86_64.rpm"],
)

rpm(
    name = "libsigsegv-0__2.13-4.el9.x86_64",
    sha256 = "931bd0ec7050e8c3b37a9bfb489e30af32486a3c77203f1e9113eeceaa3b0a3a",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsigsegv-2.13-4.el9.x86_64.rpm"],
)

rpm(
    name = "libslirp-0__4.4.0-4.el9.x86_64",
    sha256 = "06a12c4b78f60bd866ea91e648b86f1d52369f1981b5f18b6d2880ab8a951f81",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libslirp-4.4.0-4.el9.x86_64.rpm"],
)

rpm(
    name = "libsmartcols-0__2.37.4-9.el9.x86_64",
    sha256 = "ef59bdcffeaab46c8151ad3f36251d56d6b3aae7706f864c502965e6be099733",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libsmartcols-2.37.4-9.el9.x86_64.rpm"],
)

rpm(
    name = "libsndfile-0__1.0.31-7.el9.x86_64",
    sha256 = "200229d3c13ac163641d1d13a124f7c2dc63be597629c4dfd10f1e8b2b324573",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libsndfile-1.0.31-7.el9.x86_64.rpm"],
)

rpm(
    name = "libsoup-0__2.72.0-8.el9.x86_64",
    sha256 = "f28214b594a46422e75a946a491de3f8cf29289c33c26ecab60cce82fcff6d68",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libsoup-2.72.0-8.el9.x86_64.rpm"],
)

rpm(
    name = "libssh-0__0.10.4-6.el9.x86_64",
    sha256 = "64398275cda16447dfaf7bc50815ccab33e18e8f9b6d6dd2a14d4ff8d69a11e9",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libssh-0.10.4-6.el9.x86_64.rpm"],
)

rpm(
    name = "libssh-config-0__0.10.4-6.el9.x86_64",
    sha256 = "2b46d2dc134c9dfcd08c9f9c8630cade0bf3741c2e91f2ec075ffaffe6957adc",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libssh-config-0.10.4-6.el9.noarch.rpm"],
)

rpm(
    name = "libstdc__plus____plus__-0__11.3.1-4.3.el9.x86_64",
    sha256 = "5efc25ffd3cc5822f39298226d1cbf4a42e39da6ede920842e1333c1bf1055cb",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libstdc++-11.3.1-4.3.el9.x86_64.rpm"],
)

rpm(
    name = "libtasn1-0__4.16.0-8.el9.x86_64",
    sha256 = "c8b13c9e1292de474e76ab80f230f86cce2e8f5f53592e168bdcaa604ed1b37d",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libtasn1-4.16.0-8.el9.x86_64.rpm"],
)

rpm(
    name = "libtirpc-0__1.3.3-1.el9.x86_64",
    sha256 = "a8e744f25465ade2ebfbda123e1f9b6db6caa02747aa7274f90bcc3c7599f17b",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libtirpc-1.3.3-1.el9.x86_64.rpm"],
)

rpm(
    name = "libtpms-0__0.8.2-0.20210301git729fc6a4ca.el9.6.x86_64",
    sha256 = "0f20d5977b5eb078a892231d83ee0b2ce74734216502371e276d8a1c5615679d",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libtpms-0.8.2-0.20210301git729fc6a4ca.el9.6.x86_64.rpm"],
)

rpm(
    name = "libunistring-0__0.9.10-15.el9.x86_64",
    sha256 = "11e736e44265d2d0ca0afa4c11cfe0856553c4124e534fb616e6ab61c9b59e46",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libunistring-0.9.10-15.el9.x86_64.rpm"],
)

rpm(
    name = "libusbx-0__1.0.26-1.el9.x86_64",
    sha256 = "bfc8e2bfbcc0e6aaa4e4e665e52ebdc93fb84f7bf00be4640df0fa6df9cbf042",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libusbx-1.0.26-1.el9.x86_64.rpm"],
)

rpm(
    name = "libutempter-0__1.2.1-6.el9.x86_64",
    sha256 = "fab361a9cba04490fd8b5664049983d1e57ebf7c1080804726ba600708524125",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libutempter-1.2.1-6.el9.x86_64.rpm"],
)

rpm(
    name = "libuuid-0__2.37.4-9.el9.x86_64",
    sha256 = "73b06bf582fb3e0161e55714040e9e0c44d81099dc17485bacaf8c30d3fab4e7",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libuuid-2.37.4-9.el9.x86_64.rpm"],
)

rpm(
    name = "libverto-0__0.3.2-3.el9.x86_64",
    sha256 = "c55578b84f169c4ed79b2d50ea03fd1817007e35062c9fe7a58e6cad025f3b24",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libverto-0.3.2-3.el9.x86_64.rpm"],
)

rpm(
    name = "libverto-libev-0__0.3.2-3.el9.x86_64",
    sha256 = "7d4423bc582773e23bf08f1f73d99275838a45fa188971a2f20448811e524a50",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libverto-libev-0.3.2-3.el9.x86_64.rpm"],
)

rpm(
    name = "libvirt-client-0__8.10.0-2.el9.x86_64",
    sha256 = "0f5b76b6627f225c0cfd2255ec813834e7a817a8720608726f95288af1b40dd1",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libvirt-client-8.10.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-0__8.10.0-2.el9.x86_64",
    sha256 = "94ca0e49a8c5ac5ec25b2fd278c828894037cd1cb4c22029b019038222c44084",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libvirt-daemon-8.10.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-qemu-0__8.10.0-2.el9.x86_64",
    sha256 = "d1e0f4ad9a062d53c7be9eee696afe88d06ca6a3b30a108df0cdd5acbf378b0d",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-qemu-8.10.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-secret-0__8.10.0-2.el9.x86_64",
    sha256 = "82f36544420b5db9d9b34b2477d04918ae3f3e27105c39ebe3a7c5a9c15f9a10",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-secret-8.10.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "libvirt-daemon-driver-storage-core-0__8.10.0-2.el9.x86_64",
    sha256 = "6336aaf0181affcb996613fae33154dfd68ed69b4cbefb99d396d2d7c29bd48e",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libvirt-daemon-driver-storage-core-8.10.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "libvirt-libs-0__8.10.0-2.el9.x86_64",
    sha256 = "95fd37c3743670f6a6af73dcd0494796311f3febe5d2c891f964d7bd81ea2f44",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libvirt-libs-8.10.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "libvorbis-1__1.3.7-5.el9.x86_64",
    sha256 = "b6566ca8045af971aa48ca65327e183a7bc4f6ec59f36db2de26a6caa2f87074",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libvorbis-1.3.7-5.el9.x86_64.rpm"],
)

rpm(
    name = "libwayland-client-0__1.21.0-1.el9.x86_64",
    sha256 = "2b4a3e9acef0b0967f962e960f0c87f6f7cd51aa04262ab2ecf2ab58173d80c6",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libwayland-client-1.21.0-1.el9.x86_64.rpm"],
)

rpm(
    name = "libwayland-server-0__1.21.0-1.el9.x86_64",
    sha256 = "ebd8ae6e3ce81c785ab72d60b5317b8b376340a0bf5be460be3245368465d619",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libwayland-server-1.21.0-1.el9.x86_64.rpm"],
)

rpm(
    name = "libxcb-0__1.13.1-9.el9.x86_64",
    sha256 = "569018774aeb89760ade7d49c35bc1489ed0fdc3ddd6a2858f7a56811485c93f",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libxcb-1.13.1-9.el9.x86_64.rpm"],
)

rpm(
    name = "libxcrypt-0__4.4.18-3.el9.x86_64",
    sha256 = "97e88678b420f619a44608fff30062086aa1dd6931ecbd54f21bba005ff1de1a",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libxcrypt-4.4.18-3.el9.x86_64.rpm"],
)

rpm(
    name = "libxcrypt-compat-0__4.4.18-3.el9.x86_64",
    sha256 = "3ea916c72412d3a7efd8c70cfa1ed18863c018091001b631390b19c454136b87",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libxcrypt-compat-4.4.18-3.el9.x86_64.rpm"],
)

rpm(
    name = "libxkbcommon-0__1.0.3-4.el9.x86_64",
    sha256 = "240837601b4cb9260b28f66e39ad45c889e27902b4a80b36a25532c0a19ccf14",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libxkbcommon-1.0.3-4.el9.x86_64.rpm"],
)

rpm(
    name = "libxml2-0__2.9.13-3.el9.x86_64",
    sha256 = "fb8e9a41956d07af0749b921e8c625311877b3257430d149e1903bcd16899f41",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libxml2-2.9.13-3.el9.x86_64.rpm"],
)

rpm(
    name = "libxshmfence-0__1.3-10.el9.x86_64",
    sha256 = "a9681af3e5e80d7f099641ac7a37bdb36d929e897152a6490856f5461831cd5e",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libxshmfence-1.3-10.el9.x86_64.rpm"],
)

rpm(
    name = "libxslt-0__1.1.34-9.el9.x86_64",
    sha256 = "576a1d36454a155d109ba1d0bb89b3a90b932d0b539fcd6392a67054bebc0015",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/libxslt-1.1.34-9.el9.x86_64.rpm"],
)

rpm(
    name = "libyaml-0__0.2.5-7.el9.x86_64",
    sha256 = "e939227fdb6a25c742b03ac88ce4f7dc5738f36ff3aac29a4acf37949c8fcb27",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libyaml-0.2.5-7.el9.x86_64.rpm"],
)

rpm(
    name = "libzstd-0__1.5.1-2.el9.x86_64",
    sha256 = "0840678cb3c1b418286f55da6973df9468c4cf500192de82d05ef28e6b4215a0",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/libzstd-1.5.1-2.el9.x86_64.rpm"],
)

rpm(
    name = "llvm-libs-0__15.0.1-1.el9.x86_64",
    sha256 = "7becbba4aa04dfe0d18ff8bb384ff86479e74eae6d6fa7e7715490a982b48247",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/llvm-libs-15.0.1-1.el9.x86_64.rpm"],
)

rpm(
    name = "lua-libs-0__5.4.2-7.el9.x86_64",
    sha256 = "dfcd4c7262dd2217eee295b0742d9556859091c5118888784eaf6de945029566",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/lua-libs-5.4.2-7.el9.x86_64.rpm"],
)

rpm(
    name = "lua-srpm-macros-0__1-6.el9.x86_64",
    sha256 = "03da222b0be73674c1ebcda158bcca69db8ac2892b15cc858a05b4d849e373b5",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/lua-srpm-macros-1-6.el9.noarch.rpm"],
)

rpm(
    name = "lz4-libs-0__1.9.3-5.el9.x86_64",
    sha256 = "cba6a63054d070956a182e33269ee245bcfbe87e3e605c27816519db762a66ad",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/lz4-libs-1.9.3-5.el9.x86_64.rpm"],
)

rpm(
    name = "lzo-0__2.10-7.el9.x86_64",
    sha256 = "7bee77c82bd6c183bba7a4b4fdd3ecb99d0a089a25c735ebbabc44e0c51e4b2e",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/lzo-2.10-7.el9.x86_64.rpm"],
)

rpm(
    name = "lzop-0__1.04-8.el9.x86_64",
    sha256 = "ad84787d14a62195822ea89cec0fcf475f09b425f0822ce34d858d2d8bbd9466",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/lzop-1.04-8.el9.x86_64.rpm"],
)

rpm(
    name = "man-db-0__2.9.3-7.el9.x86_64",
    sha256 = "e4a4eb0d0bebce32e25ea978a2a25624c8ed6b10bd7e37ffbfbfb398d4bcbc9d",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/man-db-2.9.3-7.el9.x86_64.rpm"],
)

rpm(
    name = "mesa-dri-drivers-0__22.3.0__tilde__rc4-1.el9.x86_64",
    sha256 = "a8638783c604af6775ac346279dfcfedcd1a7b389b70dda114f972e6aae874de",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/mesa-dri-drivers-22.3.0~rc4-1.el9.x86_64.rpm"],
)

rpm(
    name = "mesa-filesystem-0__22.3.0__tilde__rc4-1.el9.x86_64",
    sha256 = "b451ae75da9e3bf04d3a2b8ec3be379dbd5b78d37d940c5337996f9555dc8b38",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/mesa-filesystem-22.3.0~rc4-1.el9.x86_64.rpm"],
)

rpm(
    name = "mesa-libEGL-0__22.3.0__tilde__rc4-1.el9.x86_64",
    sha256 = "655fb7f70262405bb2b74f5aa7e04f96dfa25835d3a67dfee0d1142b7aaf6435",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/mesa-libEGL-22.3.0~rc4-1.el9.x86_64.rpm"],
)

rpm(
    name = "mesa-libGL-0__22.3.0__tilde__rc4-1.el9.x86_64",
    sha256 = "b3328469b5054c7af3b9c7edd97fe4d409259d8603c713cbcd0c99893416bb6b",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/mesa-libGL-22.3.0~rc4-1.el9.x86_64.rpm"],
)

rpm(
    name = "mesa-libgbm-0__22.3.0__tilde__rc4-1.el9.x86_64",
    sha256 = "e12032d582bd9cae8145f064bda1ebc7b31749d4b2fb898c2ba3a23e6fdfbc5e",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/mesa-libgbm-22.3.0~rc4-1.el9.x86_64.rpm"],
)

rpm(
    name = "mesa-libglapi-0__22.3.0__tilde__rc4-1.el9.x86_64",
    sha256 = "e4cff3c5d9a6e3f6b4e8b52cb03092fd8d8e485b8c2be165f0ee113e4d1f9bb9",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/mesa-libglapi-22.3.0~rc4-1.el9.x86_64.rpm"],
)

rpm(
    name = "mingw-filesystem-base-0__139-1.el9.x86_64",
    sha256 = "78fe932d003e611f05f5ea4a3676b47313a69c5ec605c142c151c9b0f11546d8",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/mingw-filesystem-base-139-1.el9.noarch.rpm"],
)

rpm(
    name = "mingw32-crt-0__10.0.0-2.1.el9.x86_64",
    sha256 = "0dcd7e6387b2c040aae717a65e6a985ad9c320c231eb6c25f51b89d1c9514f18",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/mingw32-crt-10.0.0-2.1.el9.noarch.rpm"],
)

rpm(
    name = "mingw32-filesystem-0__139-1.el9.x86_64",
    sha256 = "fedbecb6aebd8555e4b549d75a5249109b4e1f57c8cf6e69753309d8309589da",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/mingw32-filesystem-139-1.el9.noarch.rpm"],
)

rpm(
    name = "mingw32-srvany-0__1.0-29.20210127git89f2162c.el9.x86_64",
    sha256 = "fb6108128e39e4c0b7744f6f076bb526ec21fe9d86e2fe864dcc3fd1abb037c8",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/mingw32-srvany-1.0-29.20210127git89f2162c.el9.noarch.rpm"],
)

rpm(
    name = "mpfr-0__4.1.0-7.el9.x86_64",
    sha256 = "179760104aa5a31ca463c586d0f21f380ba4d0eed212eee91bd1ca513e5d7a8d",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/mpfr-4.1.0-7.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-basic-filters-0__1.30.8-2.el9.x86_64",
    sha256 = "6d60957d23072cd2b1efe45331638d404c579d3f991bcc17d095de2578482d78",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-basic-filters-1.30.8-2.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-basic-plugins-0__1.30.8-2.el9.x86_64",
    sha256 = "ae0cb96c8a4926bb94529bb06daccb9dda81431e8ad5fb854202160760f854fb",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-basic-plugins-1.30.8-2.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-curl-plugin-0__1.30.8-2.el9.x86_64",
    sha256 = "b5d2b65163c808f6b9a5bc4c2f01981d03db73dd38c1f502a4893b19922f976d",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-curl-plugin-1.30.8-2.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-nbd-plugin-0__1.30.8-2.el9.x86_64",
    sha256 = "1474165b24773a269abfbefb230ff9673264bd677ee378177ca9f8f203610649",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-nbd-plugin-1.30.8-2.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-python-plugin-0__1.30.8-2.el9.x86_64",
    sha256 = "2d05d7995726a48f52f922e6f5b6183a19b11c8b8c0e5ab2d2bdf8748223632a",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-python-plugin-1.30.8-2.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-server-0__1.30.8-2.el9.x86_64",
    sha256 = "ba7a6397ef65db5065300e599652c98666b149a7bcff8bef481b194e4cc339b7",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-server-1.30.8-2.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-ssh-plugin-0__1.30.8-2.el9.x86_64",
    sha256 = "d3024a1263e061603fea8eb8741209017981d387d54d352b2b44a8558da0e035",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-ssh-plugin-1.30.8-2.el9.x86_64.rpm"],
)

rpm(
    name = "nbdkit-vddk-plugin-0__1.30.8-2.el9.x86_64",
    sha256 = "787d79c9a916d241bbc1ff9980526719951a115625f777d56f709c35d70beb42",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/nbdkit-vddk-plugin-1.30.8-2.el9.x86_64.rpm"],
)

rpm(
    name = "ncurses-0__6.2-8.20210508.el9.x86_64",
    sha256 = "189e3354688fca3f3cbf1dbf0bcba5a97f0d5690690d56073853004d285aa218",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ncurses-6.2-8.20210508.el9.x86_64.rpm"],
)

rpm(
    name = "ncurses-base-0__6.2-8.20210508.el9.x86_64",
    sha256 = "e4cc4a4a479b8c27776debba5c20e8ef21dc4b513da62a25ed09f88386ac08a8",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ncurses-base-6.2-8.20210508.el9.noarch.rpm"],
)

rpm(
    name = "ncurses-libs-0__6.2-8.20210508.el9.x86_64",
    sha256 = "328f4d50e66b00f24344ebe239817204fda8e68b1d988c6943abb3c36231beaa",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ncurses-libs-6.2-8.20210508.el9.x86_64.rpm"],
)

rpm(
    name = "ndctl-libs-0__71.1-8.el9.x86_64",
    sha256 = "69d469e5106559ca5a156a2191f85e89fd44f7866701bfb35e197e5133413098",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/ndctl-libs-71.1-8.el9.x86_64.rpm"],
)

rpm(
    name = "nettle-0__3.8-3.el9.x86_64",
    sha256 = "ed956f9e018ab00d6ddf567487dd6bbcdc634d27dd69b485b416c6cf40026b82",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/nettle-3.8-3.el9.x86_64.rpm"],
)

rpm(
    name = "nfs-utils-1__2.5.4-17.el9.x86_64",
    sha256 = "377f5458197503e4d2c2414df8580a499f125b99b8c22fcf8278bed8c3121edd",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/nfs-utils-2.5.4-17.el9.x86_64.rpm"],
)

rpm(
    name = "npth-0__1.6-8.el9.x86_64",
    sha256 = "a7da4ef003bc60045bc60dae299b703e7f1db326f25208fb922ce1b79e2882da",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/npth-1.6-8.el9.x86_64.rpm"],
)

rpm(
    name = "numactl-libs-0__2.0.14-7.el9.x86_64",
    sha256 = "7a3bc16b3fee48c53e0f54a7cb4cd3857eb1be3984d58da3bdf2c297d6b55af1",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/numactl-libs-2.0.14-7.el9.x86_64.rpm"],
)

rpm(
    name = "numad-0__0.5-36.20150602git.el9.x86_64",
    sha256 = "1b4242cdefa165b70926aee4dd4606b0f5ecdf4a436812746e9fe1c417724d23",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/numad-0.5-36.20150602git.el9.x86_64.rpm"],
)

rpm(
    name = "ocaml-srpm-macros-0__6-6.el9.x86_64",
    sha256 = "2f2da4857b7278051f518d1b5d5158f23025c778e77f8284a48923e7e9dacd92",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/ocaml-srpm-macros-6-6.el9.noarch.rpm"],
)

rpm(
    name = "openblas-srpm-macros-0__2-11.el9.x86_64",
    sha256 = "d3a8a6bbf4e7bdd37bb608dfdae85b6f311a0c0f176d51919e88f3268cd204d4",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/openblas-srpm-macros-2-11.el9.noarch.rpm"],
)

rpm(
    name = "openldap-0__2.6.2-3.el9.x86_64",
    sha256 = "8ce2a645dfc4444c698d8c2a644df93fd53b9a00ef887e138528aa473ee76456",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openldap-2.6.2-3.el9.x86_64.rpm"],
)

rpm(
    name = "openssh-0__8.7p1-24.el9.x86_64",
    sha256 = "44240d128468dd6fdd06b3a59d748b42679e2a17bc284bf458c040c389500652",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssh-8.7p1-24.el9.x86_64.rpm"],
)

rpm(
    name = "openssh-clients-0__8.7p1-24.el9.x86_64",
    sha256 = "13c118125bc7079d89bceddce8eb813d13b96d393c2945128731eed593fa40a3",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssh-clients-8.7p1-24.el9.x86_64.rpm"],
)

rpm(
    name = "openssl-1__3.0.7-2.el9.x86_64",
    sha256 = "4887f8c961fd4415be99297dded50e2796f82fb631af74a8967d5fb6df5978de",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-3.0.7-2.el9.x86_64.rpm"],
)

rpm(
    name = "openssl-libs-1__3.0.7-2.el9.x86_64",
    sha256 = "6c4812a785e5a662ae74c1f45e2e9b4ca456c7a083cd9ae17db087c869d7aff3",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/openssl-libs-3.0.7-2.el9.x86_64.rpm"],
)

rpm(
    name = "opus-0__1.3.1-10.el9.x86_64",
    sha256 = "d194718353f0874b9f85327821fc45adba85f646474b473d1b455b9075a77ae1",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/opus-1.3.1-10.el9.x86_64.rpm"],
)

rpm(
    name = "osinfo-db-0__20221130-1.el9.x86_64",
    sha256 = "21c7afdb9f8180fcf0440109933fc858236f9a51a477c39e25cda7270bf2f9b8",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/osinfo-db-20221130-1.el9.noarch.rpm"],
)

rpm(
    name = "osinfo-db-tools-0__1.10.0-1.el9.x86_64",
    sha256 = "2681f49bf19314e44e7189852d6fbfc22fc3ed428240df9f3936a5200c14ddd0",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/osinfo-db-tools-1.10.0-1.el9.x86_64.rpm"],
)

rpm(
    name = "ovirt-imageio-common-0__2.4.7-1.el9.x86_64",
    sha256 = "a50c13b4734da8472a646a0811a8aa11bb38ce19b2a206f5403e8d3798597be5",
    urls = ["https://mirror.stream.centos.org/SIGs/9-stream/virt/x86_64/ovirt-45/Packages/o/ovirt-imageio-common-2.4.7-1.el9.x86_64.rpm"],
)

rpm(
    name = "p11-kit-0__0.24.1-2.el9.x86_64",
    sha256 = "da167e41efd19cf25fd1c708b6f123d0203824324b14dd32401d49f2aa0ef0a6",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/p11-kit-0.24.1-2.el9.x86_64.rpm"],
)

rpm(
    name = "p11-kit-trust-0__0.24.1-2.el9.x86_64",
    sha256 = "ae9a633c58980328bef6358c6aa3c9ce0a65130c66fbfa4249922ddf5a3e2bb1",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/p11-kit-trust-0.24.1-2.el9.x86_64.rpm"],
)

rpm(
    name = "pam-0__1.5.1-14.el9.x86_64",
    sha256 = "c4d8be2502028e700815c3c80a9cd4c23618ae70a6b9af27a9996c1f9b3b93c8",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pam-1.5.1-14.el9.x86_64.rpm"],
)

rpm(
    name = "parted-0__3.5-2.el9.x86_64",
    sha256 = "ab6500203b5f0b3bd551c026ca60e5aec51170bdc62978a2702d386d2a645b5e",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/parted-3.5-2.el9.x86_64.rpm"],
)

rpm(
    name = "pcre-0__8.44-3.el9.3.x86_64",
    sha256 = "4a3cb61eb08c4f24e44756b6cb329812fe48d5c65c1fba546fadfa975045a8c5",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre-8.44-3.el9.3.x86_64.rpm"],
)

rpm(
    name = "pcre2-0__10.40-2.el9.x86_64",
    sha256 = "8cc83f9f130e6ef50d54d75eb4050ce879d8acaf5bb616b398ad92c1ad2b3d21",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre2-10.40-2.el9.x86_64.rpm"],
)

rpm(
    name = "pcre2-syntax-0__10.40-2.el9.x86_64",
    sha256 = "4dad144194fe6794c7621c38b6a7f917a81ceaeb3f2be25833b9b0af1181ebe2",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pcre2-syntax-10.40-2.el9.noarch.rpm"],
)

rpm(
    name = "perl-Carp-0__1.50-460.el9.x86_64",
    sha256 = "f1ca6aaa47ef96d6b47f20f3a2df2ce530228790f2c0330ece567cc77ddd5063",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Carp-1.50-460.el9.noarch.rpm"],
)

rpm(
    name = "perl-Class-Struct-0__0.66-479.el9.x86_64",
    sha256 = "1cfb719169e88332200502321bb2d6ba6a092e24efebd3ae9fcfa704eebac487",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Class-Struct-0.66-479.el9.noarch.rpm"],
)

rpm(
    name = "perl-Encode-4__3.08-462.el9.x86_64",
    sha256 = "85db7859711b30f268305ad0b3cdab68c8072fdf6ba60725a49657d6ae001bea",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Encode-3.08-462.el9.x86_64.rpm"],
)

rpm(
    name = "perl-Errno-0__1.30-479.el9.x86_64",
    sha256 = "b2d4b49f03caf25618ec53fca4f7fdba481fe17d43914ceca4cde962dd0518de",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Errno-1.30-479.el9.x86_64.rpm"],
)

rpm(
    name = "perl-Exporter-0__5.74-461.el9.x86_64",
    sha256 = "1fefc5a7bc8cd31a853c090cdaa0758344cacc56561532dfef20ab70bd30bcab",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Exporter-5.74-461.el9.noarch.rpm"],
)

rpm(
    name = "perl-Fcntl-0__1.13-479.el9.x86_64",
    sha256 = "98cdebdbd5a2c212bf78c06fd32793490160cd5a699d37949af91f27cb6e00f6",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Fcntl-1.13-479.el9.x86_64.rpm"],
)

rpm(
    name = "perl-File-Basename-0__2.85-479.el9.x86_64",
    sha256 = "2ec3d80d08e7468dfeeec00d598c3015a3c0af2abc56b05a57557bcc84b01146",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-File-Basename-2.85-479.el9.noarch.rpm"],
)

rpm(
    name = "perl-File-Path-0__2.18-4.el9.x86_64",
    sha256 = "74b7f75cf3c8bf7191a8e6d88689d7aca1b1f60d56c890ead7d558a704a4a4cc",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-File-Path-2.18-4.el9.noarch.rpm"],
)

rpm(
    name = "perl-File-Temp-1__0.231.100-4.el9.x86_64",
    sha256 = "a1416670c051fdf7ea5e7ffac059d88e17b14a61cf75a95be6e3c6d2e730101b",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-File-Temp-0.231.100-4.el9.noarch.rpm"],
)

rpm(
    name = "perl-File-stat-0__1.09-479.el9.x86_64",
    sha256 = "362bb253991b67fd82c6063483f2a2cb32b3c0198e9e42f7f8f8597433fe350b",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-File-stat-1.09-479.el9.noarch.rpm"],
)

rpm(
    name = "perl-Getopt-Long-1__2.52-4.el9.x86_64",
    sha256 = "0053d63a5eb0bc399e2e56a2599a1a09dca20c5bcac36f713a56fec46abac391",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Getopt-Long-2.52-4.el9.noarch.rpm"],
)

rpm(
    name = "perl-Getopt-Std-0__1.12-479.el9.x86_64",
    sha256 = "c1c61c0ebb588d0314ecba36879b53f2d7f0fc84375724bb282a48697be81bf5",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Getopt-Std-1.12-479.el9.noarch.rpm"],
)

rpm(
    name = "perl-HTTP-Tiny-0__0.076-460.el9.x86_64",
    sha256 = "368b2e96ca6be6b79d0e3b4f580c1a73edd6ea966af40b7ffdaacb2fdeda3e62",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-HTTP-Tiny-0.076-460.el9.noarch.rpm"],
)

rpm(
    name = "perl-IO-0__1.43-479.el9.x86_64",
    sha256 = "49a307a8c5d91612a849c6668d9f4758c802a1ac78c501b141d11d96175d168b",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-IO-1.43-479.el9.x86_64.rpm"],
)

rpm(
    name = "perl-IPC-Open3-0__1.21-479.el9.x86_64",
    sha256 = "b33971afed2b744ce6f578d5c8c691099439d708869730f4cab7199ddc905c94",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-IPC-Open3-1.21-479.el9.noarch.rpm"],
)

rpm(
    name = "perl-MIME-Base64-0__3.16-4.el9.x86_64",
    sha256 = "ce4d4eeb8e4524212437ac964d9f0b6ba562c7064a7bf976bec5836e87e20cd2",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-MIME-Base64-3.16-4.el9.x86_64.rpm"],
)

rpm(
    name = "perl-POSIX-0__1.94-479.el9.x86_64",
    sha256 = "2ff8aa4bfa8263dafdabb8858f63ff50d86dc6894dd34edcf2b1a6a7a11c8752",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-POSIX-1.94-479.el9.x86_64.rpm"],
)

rpm(
    name = "perl-PathTools-0__3.78-461.el9.x86_64",
    sha256 = "0ef613a8fe3d9e2355a7b063f1cd5d019772c837e2435c9efcd7ea925ad27024",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-PathTools-3.78-461.el9.x86_64.rpm"],
)

rpm(
    name = "perl-Pod-Escapes-1__1.07-460.el9.x86_64",
    sha256 = "c32ad4f02ecad264d2337837848706cc44f0502f39d978a6c055166fcf8ce917",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Pod-Escapes-1.07-460.el9.noarch.rpm"],
)

rpm(
    name = "perl-Pod-Perldoc-0__3.28.01-461.el9.x86_64",
    sha256 = "fb38583786c1d851fe160de0e436ae2bb332f60e072a6e58db02b2affb9aab6c",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Pod-Perldoc-3.28.01-461.el9.noarch.rpm"],
)

rpm(
    name = "perl-Pod-Simple-1__3.42-4.el9.x86_64",
    sha256 = "4b70575a9f0ebc0f0026681112532243b0699ca7dc5dcb165cef856adc1ec0d8",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Pod-Simple-3.42-4.el9.noarch.rpm"],
)

rpm(
    name = "perl-Pod-Usage-4__2.01-4.el9.x86_64",
    sha256 = "2dded1efa254118c646affd59615237825f00e36b86a96a98fd59c8b6015612a",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Pod-Usage-2.01-4.el9.noarch.rpm"],
)

rpm(
    name = "perl-Scalar-List-Utils-4__1.56-461.el9.x86_64",
    sha256 = "f5c5348ff66fbb760cd67e25a0b71e3b0ef1463af3acd43e33d2f5e8ad3d2ce4",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Scalar-List-Utils-1.56-461.el9.x86_64.rpm"],
)

rpm(
    name = "perl-SelectSaver-0__1.02-479.el9.x86_64",
    sha256 = "f15acb9fda15f0bb8cdd37d3763af70510554bf5f1cc99b679af94ee1065a74a",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-SelectSaver-1.02-479.el9.noarch.rpm"],
)

rpm(
    name = "perl-Socket-4__2.031-4.el9.x86_64",
    sha256 = "356cc16228b97a64af8b2548661ab69277b32a30663da29727ee62a2503c3cec",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Socket-2.031-4.el9.x86_64.rpm"],
)

rpm(
    name = "perl-Storable-1__3.21-460.el9.x86_64",
    sha256 = "027c042137acf66ded0de52859610be36373ccd555dc30a865a31267c22c5579",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Storable-3.21-460.el9.x86_64.rpm"],
)

rpm(
    name = "perl-Symbol-0__1.08-479.el9.x86_64",
    sha256 = "02186ed7d490b5d403c43282b8f448de509db85188c260378c5fb2cbe67a4353",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Symbol-1.08-479.el9.noarch.rpm"],
)

rpm(
    name = "perl-Term-ANSIColor-0__5.01-461.el9.x86_64",
    sha256 = "d4e87c795780fa190baba5291c390e9af304e4f134e4aa4f2d03fc9b91ca5d60",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Term-ANSIColor-5.01-461.el9.noarch.rpm"],
)

rpm(
    name = "perl-Term-Cap-0__1.17-460.el9.x86_64",
    sha256 = "5c38c53e562f24cbdb395b11358efaaf6aee82ccd385055c5ffcf1843593d8da",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Term-Cap-1.17-460.el9.noarch.rpm"],
)

rpm(
    name = "perl-Text-ParseWords-0__3.30-460.el9.x86_64",
    sha256 = "1264dd35d5deda51b4431955b2838cd18e0012dc27f0e0cb65061324216ff22c",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Text-ParseWords-3.30-460.el9.noarch.rpm"],
)

rpm(
    name = "perl-Text-Tabs__plus__Wrap-0__2013.0523-460.el9.x86_64",
    sha256 = "11966231d2834b2a9c2c0bf2af231e1257ce030da1cfecdd16d08a6a23222b24",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Text-Tabs+Wrap-2013.0523-460.el9.noarch.rpm"],
)

rpm(
    name = "perl-Time-Local-2__1.300-7.el9.x86_64",
    sha256 = "53f9616f9c60c8fb7befeff87f5b76c61ee75686a6c289f394c4793d3692de3b",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-Time-Local-1.300-7.el9.noarch.rpm"],
)

rpm(
    name = "perl-constant-0__1.33-461.el9.x86_64",
    sha256 = "6a25cfb9d83c69bd767be0e30de26dbc9ece902c9e848965c3378ef1405ceb6e",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-constant-1.33-461.el9.noarch.rpm"],
)

rpm(
    name = "perl-if-0__0.60.800-479.el9.x86_64",
    sha256 = "7e961a7b4eeff849489a8e9a26f32906b3aa5dc2273573fe41a01a2f9cc4fd20",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-if-0.60.800-479.el9.noarch.rpm"],
)

rpm(
    name = "perl-interpreter-4__5.32.1-479.el9.x86_64",
    sha256 = "c14474296b7c357589c657701431b51fd398b6e0b674094b5d39633fccc9d16e",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-interpreter-5.32.1-479.el9.x86_64.rpm"],
)

rpm(
    name = "perl-libs-4__5.32.1-479.el9.x86_64",
    sha256 = "01b2663ccf0ff5f7bbb790c04cc32ccf1c9cf5c8b43bee6e87121c693ae6217b",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-libs-5.32.1-479.el9.x86_64.rpm"],
)

rpm(
    name = "perl-mro-0__1.23-479.el9.x86_64",
    sha256 = "4ce7bded92bcb245cc9b661e9c345dc0f4fe0ed21928504db4974e72cb6bc62d",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-mro-1.23-479.el9.x86_64.rpm"],
)

rpm(
    name = "perl-overload-0__1.31-479.el9.x86_64",
    sha256 = "2e2013098c4a46adaa2d3531c23c9328b5b7b960d8d9135c50d6d0fbde4800b3",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-overload-1.31-479.el9.noarch.rpm"],
)

rpm(
    name = "perl-overloading-0__0.02-479.el9.x86_64",
    sha256 = "36081cad622d6c897c3b299c903ca652729861019b7241045b5ae2b7b03a8a57",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-overloading-0.02-479.el9.noarch.rpm"],
)

rpm(
    name = "perl-parent-1__0.238-460.el9.x86_64",
    sha256 = "342a7b84a44cd59bea045f679309ef6145a235897dedb8a9afd2d015bc17f72a",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-parent-0.238-460.el9.noarch.rpm"],
)

rpm(
    name = "perl-podlators-1__4.14-460.el9.x86_64",
    sha256 = "aac9b4c1ccd942afaec19299880eb89e1889b4531e7cab751c3be6a66f9c5fa6",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-podlators-4.14-460.el9.noarch.rpm"],
)

rpm(
    name = "perl-srpm-macros-0__1-41.el9.x86_64",
    sha256 = "22a4b51f8e870b4f8bde9ba671bc7386d9eab0cdd82c0b724d0de958fd6daab5",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-srpm-macros-1-41.el9.noarch.rpm"],
)

rpm(
    name = "perl-subs-0__1.03-479.el9.x86_64",
    sha256 = "b7df2f1b732dd9c6ae1edba61e11d3c1da9003af529130ea98c62980147104e8",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-subs-1.03-479.el9.noarch.rpm"],
)

rpm(
    name = "perl-vars-0__1.05-479.el9.x86_64",
    sha256 = "2f19f01a6382b276a1930b95080a69a6734b6cf54b4f4b67661852d4aeb25e40",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/perl-vars-1.05-479.el9.noarch.rpm"],
)

rpm(
    name = "pixman-0__0.40.0-5.el9.x86_64",
    sha256 = "8673872772fec90180fa9688363b4d808c5d01bd9951afaddfa7e64bb7274aba",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/pixman-0.40.0-5.el9.x86_64.rpm"],
)

rpm(
    name = "pkgconf-0__1.7.3-10.el9.x86_64",
    sha256 = "2ff8b131570687e4eca9877feaa9058ef7c0772cff507c019f6c26aff126d065",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pkgconf-1.7.3-10.el9.x86_64.rpm"],
)

rpm(
    name = "pkgconf-m4-0__1.7.3-10.el9.x86_64",
    sha256 = "de4946454f110a9b12ab50c9c3dfaa68633b4ae3cb4e5278b23d491eb3edc27a",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pkgconf-m4-1.7.3-10.el9.noarch.rpm"],
)

rpm(
    name = "pkgconf-pkg-config-0__1.7.3-10.el9.x86_64",
    sha256 = "e308e84f06756bf3c14bc426fb2519008ad8423925c4662bb379ea87aced19d9",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/pkgconf-pkg-config-1.7.3-10.el9.x86_64.rpm"],
)

rpm(
    name = "policycoreutils-0__3.4-4.el9.x86_64",
    sha256 = "8a43d0f8c24f1c746acae28c18232d132da6f988b022ef08d7d734f95e76b27b",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/policycoreutils-3.4-4.el9.x86_64.rpm"],
)

rpm(
    name = "policycoreutils-python-utils-0__3.4-4.el9.x86_64",
    sha256 = "6099d3d1f6db57ced2dd53011470bc29206bbecdaf1fd8b5d9b541297bdf7200",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/policycoreutils-python-utils-3.4-4.el9.noarch.rpm"],
)

rpm(
    name = "polkit-0__0.117-10.el9.x86_64",
    sha256 = "93d7128562762cf4046b849e8da6bbd65f0a31ba00c7db336976ff88d203f04f",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/polkit-0.117-10.el9.x86_64.rpm"],
)

rpm(
    name = "polkit-libs-0__0.117-10.el9.x86_64",
    sha256 = "bedb4e439852632b74834a58cdc10313dd2b0737b551ca39b7e8485ef0b02350",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/polkit-libs-0.117-10.el9.x86_64.rpm"],
)

rpm(
    name = "polkit-pkla-compat-0__0.1-21.el9.x86_64",
    sha256 = "ffb4cc04548f24cf7cd62da9747d3839af7676b29b60cfd3da59c6ec31ebdf99",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/polkit-pkla-compat-0.1-21.el9.x86_64.rpm"],
)

rpm(
    name = "popt-0__1.18-8.el9.x86_64",
    sha256 = "d864419035e99f8bb06f5d1c767608ed81f942cb128a98b590c1dbc4afbd54d4",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/popt-1.18-8.el9.x86_64.rpm"],
)

rpm(
    name = "protobuf-c-0__1.3.3-12.el9.x86_64",
    sha256 = "5d1091426fc81321e00c805fff53b2da159de91d6d219d20f3defdfde41bf1d4",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/protobuf-c-1.3.3-12.el9.x86_64.rpm"],
)

rpm(
    name = "psmisc-0__23.4-3.el9.x86_64",
    sha256 = "e02fc28d42912689b006fcc1e98bdb5b0eefba538eb024c4e00ec9adc348449d",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/psmisc-23.4-3.el9.x86_64.rpm"],
)

rpm(
    name = "publicsuffix-list-dafsa-0__20210518-3.el9.x86_64",
    sha256 = "992c17312bf5f144ec17b3c9733ab180c6c3641323d2deaf7c13e6bd1971f7a6",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/publicsuffix-list-dafsa-20210518-3.el9.noarch.rpm"],
)

rpm(
    name = "pulseaudio-libs-0__15.0-2.el9.x86_64",
    sha256 = "36faeb239a9688e7d4cd314c2fe946db8ad514553c9508415da0fbbc41279cbb",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/pulseaudio-libs-15.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "python-srpm-macros-0__3.9-52.el9.x86_64",
    sha256 = "8e8b58bb4129a400aa63c3a4a937b3f45fbbc690a094c015f9efa9ab75b223c8",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/python-srpm-macros-3.9-52.el9.noarch.rpm"],
)

rpm(
    name = "python3-0__3.9.16-1.el9.x86_64",
    sha256 = "7f21e3ee6bc5eaf4a8844440b277040e2df1a48f904afcc1c9943a2d059cee9e",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-3.9.16-1.el9.x86_64.rpm"],
)

rpm(
    name = "python3-audit-0__3.0.7-103.el9.x86_64",
    sha256 = "1a505ea785f7b7c63cb7012e156ca9ce17e6bea664e12ce6172f90c7c1bc876d",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/python3-audit-3.0.7-103.el9.x86_64.rpm"],
)

rpm(
    name = "python3-devel-0__3.9.16-1.el9.x86_64",
    sha256 = "3479b144ceb356f4c5ee786e8c176cc8a171d31783b8161a295d06fafd6d5ff1",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/python3-devel-3.9.16-1.el9.x86_64.rpm"],
)

rpm(
    name = "python3-libs-0__3.9.16-1.el9.x86_64",
    sha256 = "21a7fe05e3c1a36b8242f5c783f7cdf636634b69bbd21428089b948f9c2433bc",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-libs-3.9.16-1.el9.x86_64.rpm"],
)

rpm(
    name = "python3-libselinux-0__3.4-3.el9.x86_64",
    sha256 = "20515d5233fef0484a439380a9b1996c2ea7d21c502e445283abbc69cb2dcc73",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/python3-libselinux-3.4-3.el9.x86_64.rpm"],
)

rpm(
    name = "python3-libsemanage-0__3.4-2.el9.x86_64",
    sha256 = "8c8860b3aca234896e222cda11917c9a31b2e8b606659a628ccd12046608c97c",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/python3-libsemanage-3.4-2.el9.x86_64.rpm"],
)

rpm(
    name = "python3-ovirt-engine-sdk4-0__4.6.0-1.el9.x86_64",
    sha256 = "a8d6968bc13f5fcdbaea3a7bd446582484d912fe1972534b43a58383c5510930",
    urls = ["https://mirror.stream.centos.org/SIGs/9-stream/virt/x86_64/ovirt-45/Packages/p/python3-ovirt-engine-sdk4-4.6.0-1.el9.x86_64.rpm"],
)

rpm(
    name = "python3-pip-wheel-0__21.2.3-6.el9.x86_64",
    sha256 = "8e9e72535944204b48dbcb9cb34007b4991bdb4b5223e4c5874b07c6c122c1ff",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-pip-wheel-21.2.3-6.el9.noarch.rpm"],
)

rpm(
    name = "python3-policycoreutils-0__3.4-4.el9.x86_64",
    sha256 = "9fdb05a377d96b5d39338da164989af08224280b3410619dd38c3b1eb763aa50",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/python3-policycoreutils-3.4-4.el9.noarch.rpm"],
)

rpm(
    name = "python3-pycurl-0__7.43.0.6-8.el9.x86_64",
    sha256 = "250c5fc154b79c97e5f66514b5b2335d52e879f932c863df157094ac87fc4fd1",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/python3-pycurl-7.43.0.6-8.el9.x86_64.rpm"],
)

rpm(
    name = "python3-pyyaml-0__5.4.1-6.el9.x86_64",
    sha256 = "9328fd3bfbd7bddde47efcb68258a7952872ecfd7a216a1c448af7f5926b22da",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-pyyaml-5.4.1-6.el9.x86_64.rpm"],
)

rpm(
    name = "python3-setools-0__4.4.0-5.el9.x86_64",
    sha256 = "64141784e3df47d62dc28b7552dc5fc17ea2a5d7ef261a59e98c0502a056475f",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-setools-4.4.0-5.el9.x86_64.rpm"],
)

rpm(
    name = "python3-setuptools-0__53.0.0-11.el9.x86_64",
    sha256 = "f625c5e6f67f8a7a595664cbbab9c7c9707228851ff363151cd01175c2d67015",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-setuptools-53.0.0-11.el9.noarch.rpm"],
)

rpm(
    name = "python3-setuptools-wheel-0__53.0.0-11.el9.x86_64",
    sha256 = "b923161167a7bab6fc9f235ebe4ae0f0344df9db6f1879dc9a52fd2c1efe2af5",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-setuptools-wheel-53.0.0-11.el9.noarch.rpm"],
)

rpm(
    name = "python3-six-0__1.15.0-9.el9.x86_64",
    sha256 = "efecffed29602079a1ea1d41c819271ec705a97a68891b43e1d626b2fa0ea8a1",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/python3-six-1.15.0-9.el9.noarch.rpm"],
)

rpm(
    name = "qemu-guest-agent-17__7.2.0-2.el9.x86_64",
    sha256 = "4de1538907abfbf4fc57b9c6a5f60e761d6c4fcf5213e82dcafcd706e4fe15de",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-guest-agent-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-img-17__7.2.0-2.el9.x86_64",
    sha256 = "81909326169bc78d2325fc1de65e8be286c285b72d40421ac4397f910454a095",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-img-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-17__7.2.0-2.el9.x86_64",
    sha256 = "a2a7325199ae0ac83066a6d147ce1a43bab6de3a2bad75662fba0258dc9cf92b",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-audio-pa-17__7.2.0-2.el9.x86_64",
    sha256 = "11439e5fe97447043732829f294b07131ba501b532d47d266d74643a8fd1e9ba",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-audio-pa-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-block-rbd-17__7.2.0-2.el9.x86_64",
    sha256 = "32f3cf36eb400266b1b1dc2f18eb8b14063ae7fcbada2987f972857e5379da84",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-block-rbd-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-common-17__7.2.0-2.el9.x86_64",
    sha256 = "1636b119fa50e394fe18914d897873c5db1537398921ef1043bd0ec850432a14",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-common-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-core-17__7.2.0-2.el9.x86_64",
    sha256 = "4820e7b3aca6a358c95db8ae59de26f20c276f3e88f41beefec6fd851fad783e",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-core-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-device-display-virtio-gpu-17__7.2.0-2.el9.x86_64",
    sha256 = "36d375adf8586a3a61541551ba236b095925460768550908d11a5bbd55f04b8e",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-device-display-virtio-gpu-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-device-display-virtio-gpu-pci-17__7.2.0-2.el9.x86_64",
    sha256 = "0327f684c02b072b0b103d71bc736ec193a3afae8129ed1f9c768b3894172753",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-device-display-virtio-gpu-pci-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-device-display-virtio-vga-17__7.2.0-2.el9.x86_64",
    sha256 = "c5a63cbffd59068b743d5a18aebfd6d3143fac8aaef6fb3115b7d24378da0c53",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-device-display-virtio-vga-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-device-usb-host-17__7.2.0-2.el9.x86_64",
    sha256 = "c55047b1ea433fe2c05ecfc9622a50e6fa0cb3181d82e4e21abdb3a1b4d1db71",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-device-usb-host-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-device-usb-redirect-17__7.2.0-2.el9.x86_64",
    sha256 = "6a667a9e68512a02496d1543d07c8800768c6917e468eb4e0eaec546c166719f",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-device-usb-redirect-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-docs-17__7.2.0-2.el9.x86_64",
    sha256 = "51f7ec452e092964c9260895def5b694a99f39fa86b22dd8c54e81538cc3897e",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-docs-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-tools-17__7.2.0-2.el9.x86_64",
    sha256 = "1be543c16a85a0c5092ac2bfe3c62ecb70c9dfc33ceb48de43071e526b401161",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-tools-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-ui-egl-headless-17__7.2.0-2.el9.x86_64",
    sha256 = "b5427f0d9faa0660f49c29ed7ebd6838c1e1f58fecf474982441f5b4938cb220",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-ui-egl-headless-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-kvm-ui-opengl-17__7.2.0-2.el9.x86_64",
    sha256 = "590258824f0c55d9e60eae0ca92c0e025f860ed00809b974b481325413803ed0",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-kvm-ui-opengl-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-pr-helper-17__7.2.0-2.el9.x86_64",
    sha256 = "e1959c6322928519754cebd4e62d9a0a7923736c8eff527672d3bee2338602af",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-pr-helper-7.2.0-2.el9.x86_64.rpm"],
)

rpm(
    name = "qemu-virtiofsd-17__6.2.0-10.el9.x86_64",
    sha256 = "a28ea97c445909a6cc7fee51fe2994b16492f8bc132ba904b2b93541cca9fae2",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qemu-virtiofsd-6.2.0-10.el9.x86_64.rpm"],
)

rpm(
    name = "qt5-srpm-macros-0__5.15.3-1.el9.x86_64",
    sha256 = "a95d975dbb172b1f904501a5096eecaf3d569f72b3a8417eae7df8f6796f8a07",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/qt5-srpm-macros-5.15.3-1.el9.noarch.rpm"],
)

rpm(
    name = "quota-1__4.06-6.el9.x86_64",
    sha256 = "b4827d71208202beeecc6e661584b3cf008f2ee22ddd7250089dd94ff22be31e",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/quota-4.06-6.el9.x86_64.rpm"],
)

rpm(
    name = "quota-nls-1__4.06-6.el9.x86_64",
    sha256 = "7a63c4fcc7166563de95bfffb23b54db2b17c8cef178f5c0887ac8f5ab8ec1e3",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/quota-nls-4.06-6.el9.noarch.rpm"],
)

rpm(
    name = "readline-0__8.1-4.el9.x86_64",
    sha256 = "49945472925286ad89b0575657b43f9224777e36b442f0c88df67f0b61e26aee",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/readline-8.1-4.el9.x86_64.rpm"],
)

rpm(
    name = "redhat-rpm-config-0__197-1.el9.x86_64",
    sha256 = "7fc7085d3c30e832e71002732c405d51b90cf0f57e97408c3666df3b0e88e2f6",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/redhat-rpm-config-197-1.el9.noarch.rpm"],
)

rpm(
    name = "rpcbind-0__1.2.6-5.el9.x86_64",
    sha256 = "9ff0aa1299bb78f3c494620283cd34bcc9a1aa9f03fc902f21ba4c4c854b1e22",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/rpcbind-1.2.6-5.el9.x86_64.rpm"],
)

rpm(
    name = "rpm-0__4.16.1.3-22.el9.x86_64",
    sha256 = "8d98bb7173e5135c776ba9e02be2beec9b73f44d3a5eae04db1046a2a8c1ef90",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/rpm-4.16.1.3-22.el9.x86_64.rpm"],
)

rpm(
    name = "rpm-libs-0__4.16.1.3-22.el9.x86_64",
    sha256 = "cb46344dffa44265ec567715a0468e46d4c8ff7d1bfab104f3bf01c4e870af5a",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/rpm-libs-4.16.1.3-22.el9.x86_64.rpm"],
)

rpm(
    name = "rpm-plugin-selinux-0__4.16.1.3-22.el9.x86_64",
    sha256 = "a980579de68b90527187b903950cf5e5cf2ef99d5f12939adad1419926216771",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/rpm-plugin-selinux-4.16.1.3-22.el9.x86_64.rpm"],
)

rpm(
    name = "rust-srpm-macros-0__17-4.el9.x86_64",
    sha256 = "685d57afdee6557cf9f6b82c1127b5c53fa1ade5f26965a5856cfd3b6b8cb8b5",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/rust-srpm-macros-17-4.el9.noarch.rpm"],
)

rpm(
    name = "seabios-bin-0__1.16.1-1.el9.x86_64",
    sha256 = "bc66dda921365d3e1c99a989c4e7344bb1bebf7da34af910741dff599a2a950c",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/seabios-bin-1.16.1-1.el9.noarch.rpm"],
)

rpm(
    name = "seavgabios-bin-0__1.16.1-1.el9.x86_64",
    sha256 = "3032204d68939ad64b7f245adf578c75c9d7f8ed579cf2f06a77d4d97e57a966",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/seavgabios-bin-1.16.1-1.el9.noarch.rpm"],
)

rpm(
    name = "sed-0__4.8-9.el9.x86_64",
    sha256 = "a2c5d9a7f569abb5a592df1c3aaff0441bf827c9d0e2df0ab42b6c443dbc475f",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/sed-4.8-9.el9.x86_64.rpm"],
)

rpm(
    name = "selinux-policy-0__38.1.3-1.el9.x86_64",
    sha256 = "799a804ec796d47a2303d26383bd0c2f7a5f881f778cebdb52bea4867912bfde",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/selinux-policy-38.1.3-1.el9.noarch.rpm"],
)

rpm(
    name = "selinux-policy-targeted-0__38.1.3-1.el9.x86_64",
    sha256 = "5fcf29a44865115415380f1207e1cccddcb4ed2a3a229142e7617cbbe597e9c6",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/selinux-policy-targeted-38.1.3-1.el9.noarch.rpm"],
)

rpm(
    name = "setup-0__2.13.7-8.el9.x86_64",
    sha256 = "72bb129096cae3e61f8bb2299e65af31a9ef75acf84d341d464f48c9cb63654f",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/setup-2.13.7-8.el9.noarch.rpm"],
)

rpm(
    name = "shadow-utils-2__4.9-6.el9.x86_64",
    sha256 = "21eec2a59ddfe9976c24f8e5dcf8f8ffb4d565f4214325b88f32af935399bb93",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/shadow-utils-4.9-6.el9.x86_64.rpm"],
)

rpm(
    name = "snappy-0__1.1.8-8.el9.x86_64",
    sha256 = "10facee86b64af91b06292ca9892fd94fe5fc08c068b0baed6a0927d6a64955a",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/snappy-1.1.8-8.el9.x86_64.rpm"],
)

rpm(
    name = "sqlite-libs-0__3.34.1-6.el9.x86_64",
    sha256 = "440da6dd7ad99e29e540626efe09650add959846d00a9759f0c4a417161d911e",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/sqlite-libs-3.34.1-6.el9.x86_64.rpm"],
)

rpm(
    name = "swtpm-0__0.7.0-2.20211109gitb79fd91.el9.x86_64",
    sha256 = "58e618362f6fd9b5efdfa27c1f5bb14b4a0c498f3751d5eb9f0153bcbc671024",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/swtpm-0.7.0-2.20211109gitb79fd91.el9.x86_64.rpm"],
)

rpm(
    name = "swtpm-libs-0__0.7.0-2.20211109gitb79fd91.el9.x86_64",
    sha256 = "2d72d6e18a3feb7c66caa6c5296279ba9492111620839899b9342348d2eb4acb",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/swtpm-libs-0.7.0-2.20211109gitb79fd91.el9.x86_64.rpm"],
)

rpm(
    name = "swtpm-tools-0__0.7.0-2.20211109gitb79fd91.el9.x86_64",
    sha256 = "607d390e8078b7d3fb2f65be7ea835708471c27ed320fce4cf7cca2de7174807",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/swtpm-tools-0.7.0-2.20211109gitb79fd91.el9.x86_64.rpm"],
)

rpm(
    name = "systemd-0__252-2.el9.x86_64",
    sha256 = "ccb4665bf8524aa046d66f319abde464fc9b691b0b70ede9d38b2934e6b6a9a2",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-252-2.el9.x86_64.rpm"],
)

rpm(
    name = "systemd-container-0__252-2.el9.x86_64",
    sha256 = "0c6ac267483903db657e0a24766150dedb354ddae8a4726159e828aa19828876",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-container-252-2.el9.x86_64.rpm"],
)

rpm(
    name = "systemd-libs-0__252-2.el9.x86_64",
    sha256 = "177af4b7ff14dd8d14de0744bd6789dd69e21d07a3b18053c8c5d725b0401890",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-libs-252-2.el9.x86_64.rpm"],
)

rpm(
    name = "systemd-pam-0__252-2.el9.x86_64",
    sha256 = "e24247d9fe339f755b68dc333b814f46a24c9588885d2f0868d6bc3fea38d753",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-pam-252-2.el9.x86_64.rpm"],
)

rpm(
    name = "systemd-rpm-macros-0__252-2.el9.x86_64",
    sha256 = "8458cc44b3ad822dd5dfd831c86e1955a73bbec36d31968f3e58b5361d27dca7",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/systemd-rpm-macros-252-2.el9.noarch.rpm"],
)

rpm(
    name = "tar-2__1.34-5.el9.x86_64",
    sha256 = "b907cafd5fefcab9569d5e3c807ee00b0b2beea10d08260a951fdf537edf5c2f",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tar-1.34-5.el9.x86_64.rpm"],
)

rpm(
    name = "tzdata-0__2022g-1.el9.x86_64",
    sha256 = "f29bda149f926fd192c6d0b9cbe85c723fd7dd37d795b4e346fb3a528570fd2b",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/tzdata-2022g-1.el9.noarch.rpm"],
)

rpm(
    name = "unbound-libs-0__1.16.2-2.el9.x86_64",
    sha256 = "7b6dd4c3d907b3f2d2f5ab08ed76ee97638a2c2ebfb3a8abe4a905cb1092f23d",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/unbound-libs-1.16.2-2.el9.x86_64.rpm"],
)

rpm(
    name = "unzip-0__6.0-56.el9.x86_64",
    sha256 = "630ee10eb1eac040297a85e503b13b41164036fb5fac08ab81796d033d221bdf",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/unzip-6.0-56.el9.x86_64.rpm"],
)

rpm(
    name = "usbredir-0__0.13.0-1.el9.x86_64",
    sha256 = "1468394b6f8186a80d898e44eaeaa5ebc4c223f5e08c53ddb63c1996cc75c17f",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/usbredir-0.13.0-1.el9.x86_64.rpm"],
)

rpm(
    name = "userspace-rcu-0__0.12.1-6.el9.x86_64",
    sha256 = "119e159428dda0e194c6428da57fae87ef75cce5c7271d347fe84283a7374c03",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/userspace-rcu-0.12.1-6.el9.x86_64.rpm"],
)

rpm(
    name = "util-linux-0__2.37.4-9.el9.x86_64",
    sha256 = "3b3ae5007cbd3b14f3b9689a9a0d51752df9699c2f94b1cdf44a68d3621d8e05",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-2.37.4-9.el9.x86_64.rpm"],
)

rpm(
    name = "util-linux-core-0__2.37.4-9.el9.x86_64",
    sha256 = "f426eee17734e73378b9326cd06f9d9ac14808b96078ea709da2abb632bf4c0c",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/util-linux-core-2.37.4-9.el9.x86_64.rpm"],
)

rpm(
    name = "vim-minimal-2__8.2.2637-16.el9.x86_64",
    sha256 = "9fba13d288a8aa748f407e75ff610f6ac9e78295347f75284c849c44ab67bf44",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/vim-minimal-8.2.2637-16.el9.x86_64.rpm"],
)

rpm(
    name = "virt-v2v-1__2.0.7-7.el9.x86_64",
    sha256 = "81fbbc02d44b73ff145fa128195f7b85330e6f539ee0098a7ac3d1b3cfe78dcc",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/virt-v2v-2.0.7-7.el9.x86_64.rpm"],
)

rpm(
    name = "virtio-win-0__1.9.15-4.el9.x86_64",
    sha256 = "5c27983e7228e06192b5784dc7bb4baef3fff81d06110a890b79702b5cd9dbc4",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/virtio-win-1.9.15-4.el9.noarch.rpm"],
)

rpm(
    name = "which-0__2.21-28.el9.x86_64",
    sha256 = "26730943b9a2550b0df8f17ef155efc3c3d966a711f2d5df0e351a5962369d82",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/which-2.21-28.el9.x86_64.rpm"],
)

rpm(
    name = "xfsprogs-0__5.14.2-1.el9.x86_64",
    sha256 = "53588c8816aefeee1ea82bb3b5b98a7842aaf9ef04d6d957195a947dd51534c6",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/xfsprogs-5.14.2-1.el9.x86_64.rpm"],
)

rpm(
    name = "xkeyboard-config-0__2.33-2.el9.x86_64",
    sha256 = "ca47ef1bfc9cf8b0996ffad8c423270e84f87fb2a32386b03edadc5d38a2fdf5",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/xkeyboard-config-2.33-2.el9.noarch.rpm"],
)

rpm(
    name = "xz-0__5.2.5-8.el9.x86_64",
    sha256 = "159f0d11b5a78efa493b478b0c2df7ef42a54a9710b32dba9f94dd73eb333481",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/xz-5.2.5-8.el9.x86_64.rpm"],
)

rpm(
    name = "xz-libs-0__5.2.5-8.el9.x86_64",
    sha256 = "ff3c88297d75c51a5f8e9d2d69f8ad1eaf8347e20920b4335a3e0fc53269ad28",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/xz-libs-5.2.5-8.el9.x86_64.rpm"],
)

rpm(
    name = "yajl-0__2.1.0-21.el9.x86_64",
    sha256 = "d159334f408022942e77f67322288d13c1d575a3af54512d4310310709b644d9",
    urls = ["https://mirror.stream.centos.org/9-stream/AppStream/x86_64/os/Packages/yajl-2.1.0-21.el9.x86_64.rpm"],
)

rpm(
    name = "zip-0__3.0-33.el9.x86_64",
    sha256 = "adcd60a331a0a31ad0d36fcec522203b58151aff41689faede895a42529b3f87",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/zip-3.0-33.el9.x86_64.rpm"],
)

rpm(
    name = "zlib-0__1.2.11-35.el9.x86_64",
    sha256 = "80df42ac4be2c057e332647ca98d65b1548d4e4adf52beb75d79e79fc8e48aa7",
    urls = ["https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/os/Packages/zlib-1.2.11-35.el9.x86_64.rpm"],
)
