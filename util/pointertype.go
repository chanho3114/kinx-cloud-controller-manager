package util

import "k8s.io/utils/ptr"

// helper functions for pointer types.
func BoolPtr(b bool) *bool {
	return &b
}

func Int32Ptr(i int) *int32 {
	return ptr.To(int32(i))
}

func Int64Ptr(i int) *int64 {
	return ptr.To(int64(i))
}
