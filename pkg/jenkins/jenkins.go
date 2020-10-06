package jenkins

import (
	"fmt"

	"github.com/bndr/gojenkins"
)

func CleanupOldBuilds(url, username, password, jobName string, filter func(params map[string]string) bool) error {
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
	return nil
}
