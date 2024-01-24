env GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -a -tags netgo -ldflags '-X github.com/dynatrace/dynatrace-configuration-as-code/pkg/version.MonitoringAsCode=2.x -w -extldflags "-static"' -o ./build/one-topology-windows-amd64.exe ./cmd/monaco
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -a -tags netgo -ldflags '-X github.com/dynatrace/dynatrace-configuration-as-code/pkg/version.MonitoringAsCode=2.x -w -extldflags "-static"' -o ./build/one-topology-linux-amd64 ./cmd/monaco
cp -p ./build/one-topology-linux-amd64 ./bin/one-topology
cp -p ./build/one-topology-linux-amd64 ./bin/monaco