package util

import "strings"

func GetBaseWorkDir(rcvIdent string) string {
	return strings.ReplaceAll(rcvIdent, ".", "_")
}

func GetInstanceWorkdirName() string {
	return GetRandDirName(5)
}
