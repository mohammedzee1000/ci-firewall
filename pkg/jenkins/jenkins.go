package jenkins

import (
	"fmt"
	"k8s.io/klog/v2"
	"os"
	"strconv"

	"github.com/bndr/gojenkins"
)

func CleanupOldBuilds(url, username, password, jobName string, currentBuild int, filter func(params map[string]string) bool) error {
	klog.V(2).Infof("initializing jenkins client")
	klog.V(4).Infof("jenkins url: %s, username: %s, password %s", url, username, password)
	jenkins, err := gojenkins.CreateJenkins(nil, url, username, password).Init()
	if err != nil {
		return fmt.Errorf("unable to initialize jenkins %w", err)
	}
	klog.V(2).Infof("Getting current jenkins job")
	klog.V(3).Infof("job name %s", jobName)
	job, err := jenkins.GetJob(jobName)
	if err != nil {
		return fmt.Errorf("failed to get job %s %w", jobName, err)
	}
	klog.V(2).Infof("getting builds in jenkins job")
	buildids, err := job.GetAllBuildIds()
	if err != nil {
		return fmt.Errorf("failed to get all build ids %w", err)
	}
	for _, bid := range buildids {
		klog.V(3).Infof("validating build number %d", bid.Number)
		if bid.Number < int64(currentBuild) {
			b, err := job.GetBuild(bid.Number)
			if err != nil {
				return fmt.Errorf("failed to fetch build with %w", err)
			}
			klog.V(3).Infof("checking if build is running")
			if b.IsRunning() {
				prms := make(map[string]string)
				klog.V(3).Infof("getting jenkins job parameters")
				jparams := b.GetParameters()
				klog.V(4).Infof("job parameters look like %#v", jparams)
				for _, jpr := range jparams {
					prms[jpr.Name] = jpr.Value
				}
				if filter(prms) {
					klog.V(3).Infof("stopping build as its parameters match current build parameters")
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

func NewerBuildsExist(url, username, password, jobName string, currentBuild int, filter func(params map[string]string) bool) (bool, error) {
	klog.V(2).Infof("initializing jenkins client")
	klog.V(4).Infof("jenkins url: %s, username: %s, password %s", url, username, password)
	jenkins, err := gojenkins.CreateJenkins(nil, url, username, password).Init()
	if err != nil {
		return false, fmt.Errorf("unable to initialize jenkins %w", err)
	}
	klog.V(2).Infof("Getting current jenkins job")
	klog.V(3).Infof("job name %s", jobName)
	job, err := jenkins.GetJob(jobName)
	if err != nil {
		return false, fmt.Errorf("failed to get job %s %w", jobName, err)
	}
	klog.V(2).Infof("getting builds in jenkins job")
	buildids, err := job.GetAllBuildIds()
	if err != nil {
		return false, fmt.Errorf("failed to get all build ids %w", err)
	}
	for _, bid := range buildids {
		if bid.Number > int64(currentBuild) {
			klog.V(2).Infof("found build number larger than current, checking if its paramers match")
			klog.V(4).Infof("build number %d", bid.Number)
			b, err := job.GetBuild(bid.Number)
			if err != nil {
				return false, fmt.Errorf("failed to fetch build with %w", err)
			}
			prms := make(map[string]string)
			jparams := b.GetParameters()
			klog.V(4).Infof("job parameters look like %#v", jparams)
			for _, jpr := range jparams {
				prms[jpr.Name] = jpr.Value
			}
			if filter(prms) {
				klog.V(2).Infof("found newer build in jenkins queue whose parameters match current jobs")
				return true, nil
			}
		}
	}
	klog.V(2).Infof("no newer builds found")
	return false, nil
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
