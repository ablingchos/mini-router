VERSION --shell-out-anywhere --try 0.6

ARG BUILD_INFRA_REPO=git.woa.com/mfcn/build-infra/v5:master

FROM $BUILD_INFRA_REPO+base

# prelude_xxx 空格分割，例如： prelude_redis prelude_etcd prelude_mongo
ARG preludes="prelude_redis"

# 代码生成(protobuf, go generate 等)
code:
    DO $BUILD_INFRA_REPO+CODE