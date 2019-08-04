package main

import (
	"flag"
	log  "github.com/sirupsen/logrus"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	types   = []string{"email", "deactivation", "activation", "transaction", "customer_renew", "order_processed"}
	workers = 0
	jobSleepTime = 0
	jobs = 0

	totalCounterVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "worker",
			Subsystem: "jobs",
			Name:      "processed_total",
			Help:      "Total number of jobs processed by the workers",
		},
		[]string{"worker_id", "type"},
	)

	inflightCounterVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "worker",
			Subsystem: "jobs",
			Name:      "inflight",
			Help:      "Number of jobs inflight",
		},
		[]string{"type"},
	)

	processingTimeVec = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "worker",
			Subsystem: "jobs",
			Name:      "process_time_seconds",
			Help:      "Amount of time spent processing jobs",
		},
		[]string{"worker_id", "type"},
	)
)

func init() {
	flag.IntVar(&workers, "workers", 10, "Number of workers to use")
	flag.IntVar(&jobSleepTime, "max-job-sleep", 150, "Job maximum sleep time in miliseconds")
	flag.IntVar(&jobs, "jobs", 10000, "Jobs queue")
}

func getType() string {
	return types[rand.Int()%len(types)]
}

// main entry point for the application
func main() {
	// parse the flags
	flag.Parse()

	//////////
	// Demo of Worker Processing
	//////////

	// register with the prometheus collector
	prometheus.MustRegister(
		totalCounterVec,
		inflightCounterVec,
		processingTimeVec,
	)

	// create a channel with a 10,000 Job buffer
	jobsChannel := make(chan *Job, jobs)

	// start the job processor
	go startJobProcessor(jobsChannel)

	go createJobs(jobsChannel)

	handler := http.NewServeMux()
	handler.Handle("/metrics", promhttp.Handler())

	log.Info("Starting HTTP server on port :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}

type Job struct {
	Type  string
	Sleep time.Duration
}

// makeJob creates a new job with a random sleep time between 10 ms and 4000ms
func makeJob() *Job {
	return &Job{
		Type:  getType(),
		Sleep: time.Duration(rand.Int()%jobSleepTime+10) * time.Millisecond,
	}
}

func startJobProcessor(jobs <-chan *Job) {
	log.WithField("workers", workers).Info("Starting workers\n")
	wait := sync.WaitGroup{}
	// notify the sync group we need to wait for 10 goroutines
	wait.Add(workers)

	// start 10 works
	for i := 0; i < workers; i++ {
		go func(workerID int) {
			// start the worker
			startWorker(workerID, jobs)
			wait.Done()
		}(i)
	}

	wait.Wait()
}

func createJobs(jobs chan<- *Job) {
	for {
		// create a random job
		job := makeJob()
		// track the job in the inflight tracker
		inflightCounterVec.WithLabelValues(job.Type).Inc()
		// send the job down the channel
		jobs <- job
		// don't pile up too quickly
		time.Sleep(5 * time.Millisecond)
	}
}

// creates a worker that pulls jobs from the job channel
func startWorker(workerID int, jobs <-chan *Job) {
	for {
		select {
		// read from the job channel
		case job := <-jobs:
			startTime := time.Now()

			// fake processing the request
			time.Sleep(job.Sleep)
			log.WithFields(log.Fields{
				"worker_id": workerID,
				"job_type": job.Type,
				"duration": time.Now().Sub(startTime).Seconds(),
			}).Info("Processed job")
			// track the total number of jobs processed by the worker
			totalCounterVec.WithLabelValues(strconv.FormatInt(int64(workerID), 10), job.Type).Inc()
			// decrement the inflight tracker
			inflightCounterVec.WithLabelValues(job.Type).Dec()

			processingTimeVec.WithLabelValues(strconv.FormatInt(int64(workerID), 10), job.Type).Observe(time.Now().Sub(startTime).Seconds())
		}
	}
}


func init() {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)
}
