// Code generated by kraftkit.sh/tools/protoc-gen-go-netconn. DO NOT EDIT.
// source: machine/qemu/qmp/v7alpha2/misc.proto

package qmpv7alpha2

type StopRequest struct {
	Execute string `json:"execute" default:"stop"`
}

type StopResponse struct {
}

type ContRequest struct {
	Execute string `json:"execute" default:"cont"`
}

type ContResponse struct {
}
