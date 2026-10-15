To update the golang gRPC/protobuf files, use the following `protoc` commandline:

```
protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative levelzero.proto
# To fix bad package name
sed -i -e 's/gpu_levelzero/gpulevelzero/' levelzero.pb.go levelzero_grpc.pb.go
```

> *Note*: Running `protoc` will erase copyright header and change the package name from "gpulevelzero" to "gpu.levelzero". The header and the package name needs to be added/modified after regeneration.

