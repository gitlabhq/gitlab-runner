package runner_wrapper

// If all generated files are removed - what happens when we run tests in CI/CD - we
// need to make sure that the protobuf Go files are generated before we will call
// mockery to generate mocks. As otherwise mockery will fail with the package code
// (mainly the `server.go` file) tries to reference the
// `gitlab.com/gitlab-org/gitlab-runner/helpers/runner_wrapper/proto` package that,
// at this moment, doesn't exist.
//
// We need first to generate protobuf files :)
//
// As we generate the files with `go generate ./...` it goes alphabetically, adding
// this in a file named 000_* should ensure that these `go:generate` definitions will
// be called first.

//go:generate protoc -I ./ ./proto/wrapper.proto --go_out=./
//go:generate protoc -I ./ ./proto/wrapper.proto --go-grpc_out=./
