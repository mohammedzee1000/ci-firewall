package worker

import (
	"fmt"
	"github.com/mohammedzee1000/ci-firewall/pkg/executor"
	"github.com/mohammedzee1000/ci-firewall/pkg/messages"
	"github.com/mohammedzee1000/ci-firewall/pkg/util"
	"io"
	"k8s.io/klog/v2"
	"log"
	"time"
)

func (w *Worker) sendCancelMessage() error {
	if w.receiveQueue != nil {
		klog.V(2).Infof("sending cancel message in order to ensure requester stops following any older builds")
		return w.receiveQueue.Publish(false, messages.NewCancelMessage(w.jenkinsBuild, w.jenkinsProject))
	}
	return nil
}

//sendBuildInfo sends information about the build. Returns error in case of fail
func (w *Worker) sendBuildInfo() error {
	if w.receiveQueue != nil {
		klog.V(2).Infof("publishing build information on rcv queue")
		return w.receiveQueue.Publish(false, messages.NewBuildMessage(w.jenkinsBuild, w.jenkinsProject))
	}
	return nil
}

//printAndStreamLog prints a the logs to the PrintStreamBuffer. Returns error in case of fail
func (w *Worker) printAndStreamLog(tags []string, msg string) error {
	err := w.psb.Print(fmt.Sprintf("%v %s", tags, msg), false, w.stripANSIColor)
	if err != nil {
		return fmt.Errorf("failed to stream log message %w", err)
	}
	return nil
}

func (w *Worker) handleCommandError(tags []string, err error) error {
	err1 := w.psb.Print(fmt.Sprintf("%v Command Error %s, failing gracefully", tags, err), true, w.stripANSIColor)
	if err1 != nil {
		return fmt.Errorf("failed to stream command error message %w", err)
	}
	return nil
}

//printAndStreamInfo prints and streams an info msg
func (w *Worker) printAndStreamInfo(tags []string, info string) error {
	toprint := fmt.Sprintf("%v !!!%s!!!\n", tags, info)
	return w.psb.Print(toprint, true, w.stripANSIColor)
}

func (w *Worker) printAndStreamErrors(tags []string, errList []error) error {
	errMsg := fmt.Sprintf("%v List of errors below:\n\n", tags)
	for _, e := range errList {
		errMsg = fmt.Sprintf(" - %s\n", e.Error())
	}
	return w.psb.Print(errMsg, true, w.stripANSIColor)
}

//printAndStreamCommand print and streams a command. Returns error in case of fail
func (w *Worker) printAndStreamCommand(tags []string, cmdArgs []string) error {
	return w.psb.Print(fmt.Sprintf("%v Executing command %v\n", tags, cmdArgs), true, w.stripANSIColor)
}

//runCommand runs cmd on ex the Executor in the workDir and returns success and error
func (w *Worker) runCommand(oldSuccess bool, ex executor.Executor, workDir string, cmd []string) (bool, error) {
	ctags := ex.GetTags()
	var errList []error
	success := true
	err := w.printAndStreamCommand(ctags, cmd)
	if err != nil {
		return false, err
	}
	if oldSuccess {
		klog.V(4).Infof("injected env vars look like %#v", w.envVars)
		retryBackOff := w.retryLoopBackOff
		//keep retrying for ever incrementing retry (internal break conditions present)
		for retry := 1; ; retry++ {
			// if retry > 1 then we probably failed the last attempt
			if retry > 1 {
				success = false
				err := w.printAndStreamInfo(ctags, "attempt failed due to executor error")
				if err != nil {
					return false, err
				}
				// we want to do retry loop backoff for all attempts greater than one until the last attempt
				if retry <= w.retryLoopCount {
					retryBackOff = retryBackOff + w.retryLoopBackOff
					err := w.printAndStreamInfo(ctags, fmt.Sprintf("backing of for %s before retrying", retryBackOff))
					if err != nil {
						return false, err
					}
					time.Sleep(retryBackOff)
				}
			}
			// if the last attempt was done and was not successful, then we have failed
			// Note: this handles case where retry loop count was given as 1 as well as 1 >= 1 but success is true (see initialization above)
			if retry > w.retryLoopCount && !success {
				err = w.printAndStreamErrors(ctags, errList)
				return false, executor.NewExecutorError(fmt.Errorf("aborting %v", errList))
			}
			err = w.printAndStreamInfo(ctags, fmt.Sprintf("Attempt %d", retry))
			if err != nil {
				return false, err
			}
			rdr, err := ex.InitCommand(workDir, cmd, util.EnvMapCopy(w.envVars), w.tags)
			if err != nil {
				errList = append(errList, fmt.Errorf("failed to initialize executor %w", err))
				continue
			}
			defer func(ex executor.Executor) {
				err := ex.Close()
				if err != nil {
					log.Fatalf("failed to close executor")
				}
			}(ex)
			done := make(chan error)
			go func(done chan error) {
				for {
					data, err := rdr.ReadString('\n')
					if err != nil {
						if err != io.EOF {
							done <- fmt.Errorf("error while reading from buffer %w", err)
						}
						if len(data) > 0 {
							err := w.printAndStreamLog(ctags, data)
							if err != nil {
								done <- err
							}
						}
						break
					}
					err = w.printAndStreamLog(ctags, data)
					if err != nil {
						done <- err
					}
					time.Sleep(25 * time.Millisecond)
				}
				done <- nil
			}(done)
			// if start or wait error out, then we record the error and move on to next attempt
			err = ex.Start()
			if err != nil {
				errList = append(errList, fmt.Errorf("failed to start executing command %w", err))
				continue
			}
			err = <-done
			if err != nil {
				errList = append(errList, err)
				continue
			}
			success, err = ex.Wait()
			if err != nil {
				errList = append(errList, fmt.Errorf("failed to wait for command completion %w", err))
				continue
			}
			err = w.psb.FlushToQueue()
			if err != nil {
				return false, fmt.Errorf("failed to flush %w", err)
			}
			// if we have reached this point, then we do not have any executor errors, so return success status
			return success, nil
		}
	}
	err = w.printAndStreamInfo(ex.GetTags(), "previous command failed or skipped, skipping")
	if err != nil {
		return false, err
	}
	return false, nil
}

//sendStatusMessage sends the status message over queue, based on success value
func (w *Worker) sendStatusMessage(success bool) error {
	sm := messages.NewStatusMessage(w.jenkinsBuild, success, w.jenkinsProject)
	klog.V(2).Infof("sending status message")
	if w.receiveQueue != nil {
		return w.receiveQueue.Publish(false, sm)
	}
	return nil
}

func (w *Worker) sendFinalizeMessage() error {
	klog.V(2).Infof("sending final message")
	if w.receiveQueue != nil && w.final {
		return w.receiveQueue.Publish(false, messages.NewFinalMessage(w.jenkinsBuild, w.jenkinsProject))
	}
	return nil
}

func (w *Worker) printBuildInfo() {
	fmt.Printf("!!!Build for Kind: %s Target: %s!!!\n", w.ciMessage.Kind, w.ciMessage.Target)
}
