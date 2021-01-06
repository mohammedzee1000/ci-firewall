package jenkins

import (
	"fmt"
	"os"
	"strconv"

	"github.com/bndr/gojenkins"
)

//CleanupOldBuilds stops old jenkins jobs which are running, or else returns error
func CleanupOldBuilds(url, username, password, jobName string, currentBuild int, filter func(params map[string]string) bool) error {
	jenkins, err := gojenkins.CreateJenkins(nil, url, username, password).Init()
	if err != nil {
		return fmt.Errorf("unable to initialize jenkins %w", err)
	}
	job, err := jenkins.GetJob(jobName)
	if err != nil {
		return fmt.Errorf("failed to get job %s %w", jobName, err)
	}
	buildids, err := job.GetAllBuildIds()
	if err != nil {
		return fmt.Errorf("failed to get all build ids %w", err)
	}
	for _, bid := range buildids {
		if bid.Number != int64(currentBuild) {
			b, err := job.GetBuild(bid.Number)
			if err != nil {
				return fmt.Errorf("failed to fetch build with %w", err)
			}
			if b.IsRunning() {
				prms := make(map[string]string)
				jparams := b.GetParameters()
				for _, jpr := range jparams {
					prms[jpr.Name] = jpr.Value
				}
				if filter(prms) {
					_, err = b.Stop()
					if err != nil {
						return fmt.Errorf("unable to stop build %w", err)
					}
				}
			}
		}
	}
	return nil
}

func GetJenkinsURL() string {
	return os.Getenv("JENKINS_URL")
}

func GetJenkinsJob() string {
	return os.Getenv("JOB_NAME")
}

func GetJenkinsBuildNumber() int {
	var bn int
	bns := os.Getenv("BUILD_NUMBER")
	if bns == "" {
		bn = -1
	} else {
		bn, _ = strconv.Atoi(bns)
	}

	return bn
}
