package executor

import (
	"fmt"
	"strings"
)

const executionErrorPrefix = "failed due to executor error"

func NewExecutorError(err error) error {
	return fmt.Errorf("%s: %w", executionErrorPrefix, err)
}

func IsExecutorError(err error) bool {
	return strings.Contains(err.Error(), executionErrorPrefix)
}
