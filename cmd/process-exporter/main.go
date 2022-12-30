package main

import (
	"flag"
	"fmt"
	"github.com/ncabatoff/process-exporter/config"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/ncabatoff/fakescraper"
	common "github.com/ncabatoff/process-exporter"
	"github.com/ncabatoff/process-exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	promVersion "github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
)

// Version is set at build time use ldflags.
var version string

func printManual() {
	fmt.Print(`Usage:
  process-exporter [options] -config.path filename.yml

or 

  process-exporter [options] -procnames name1,...,nameN [-namemapping k1,v1,...,kN,vN]

The recommended option is to use a config file, but for convenience and
backwards compatibility the -procnames/-namemapping options exist as an
alternative.
 
The -children option (default:true) makes it so that any process that otherwise
isn't part of its own group becomes part of the first group found (if any) when
walking the process tree upwards.  In other words, resource usage of
subprocesses is added to their parent's usage unless the subprocess identifies
as a different group name.

Command-line process selection (procnames/namemapping):

  Every process not in the procnames list is ignored.  Otherwise, all processes
  found are reported on as a group based on the process name they share. 
  Here 'process name' refers to the value found in the second field of
  /proc/<pid>/stat, which is truncated at 15 chars.

  The -namemapping option allows assigning a group name based on a combination of
  the process name and command line.  For example, using 

    -namemapping "python2,([^/]+)\.py,java,-jar\s+([^/]+).jar" 

  will make it so that each different python2 and java -jar invocation will be
  tracked with distinct metrics.  Processes whose remapped name is absent from
  the procnames list will be ignored.  Here's an example that I run on my home
  machine (Ubuntu Xenian):

    process-exporter -namemapping "upstart,(--user)" \
      -procnames chromium-browse,bash,prometheus,prombench,gvim,upstart:-user

  Since it appears that upstart --user is the parent process of my X11 session,
  this will make all apps I start count against it, unless they're one of the
  others named explicitly with -procnames.

Config file process selection (filename.yml):

  See README.md.
` + "\n")

}

func init() {
	promVersion.Version = version
	prometheus.MustRegister(promVersion.NewCollector("process_exporter"))
}

func main() {
	var (
		listenAddress = flag.String("web.listen-address", ":9256",
			"Address on which to expose metrics and web interface.")
		metricsPath = flag.String("web.telemetry-path", "/metrics",
			"Path under which to expose metrics.")
		onceToStdoutDelay = flag.Duration("once-to-stdout-delay", 0,
			"Don't bind, just wait this much time, print the metrics once to stdout, and exit")
		procfsPath = flag.String("procfs", "/proc",
			"path to read proc data from")
		children = flag.Bool("children", false,
			"if a proc is tracked, track with it any children that aren't part of their own group")
		threads = flag.Bool("threads", true,
			"report on per-threadname metrics as well")
		smaps = flag.Bool("gather-smaps", true,
			"gather metrics from smaps file, which contains proportional resident memory size")
		configPath = flag.String("config.path", "config.yml",
			"path to YAML config file")
		tlsConfigFile = flag.String("web.config.file", "",
			"path to YAML web config file")
		recheck = flag.Bool("recheck", false,
			"recheck process names on each scrape")
		debug = flag.Bool("debug", false,
			"log debugging information to stdout")
		showVersion = flag.Bool("version", false,
			"print version information and exit")
	)
	flag.Parse()

	promlogConfig := &promlog.Config{}
	logger := promlog.New(promlogConfig)

	if *showVersion {
		fmt.Printf("%s\n", promVersion.Print("process-exporter"))
		os.Exit(0)
	}

	var matchnamer common.MatchNamer

	cfg, err := config.ReadFile(*configPath, *debug)
	if err != nil {
		log.Fatalf("error reading config file %q: %v", *configPath, err)
	}
	log.Printf("Reading metrics from %s based on %q", *procfsPath, *configPath)
	for _, m := range cfg.Matchers {
		log.Printf("using config matchnamer: %v", m)
	}
	matchnamer = cfg

	pc, err := collector.NewProcessCollector(
		collector.ProcessCollectorOption{
			ProcFSPath:  *procfsPath,
			Children:    *children,
			Threads:     *threads,
			GatherSMaps: *smaps,
			Namer:       matchnamer,
			Recheck:     *recheck,
			Debug:       *debug,
		},
	)
	if err != nil {
		log.Fatalf("Error initializing: %v", err)
	}

	prometheus.MustRegister(pc)

	if *onceToStdoutDelay != 0 {
		// We throw away the first result because that first collection primes the pump, and
		// otherwise we won't see our counter metrics.  This is specific to the implementation
		// of NamedProcessCollector.Collect().
		fscraper := fakescraper.NewFakeScraper()
		fscraper.Scrape()
		time.Sleep(*onceToStdoutDelay)
		fmt.Print(fscraper.Scrape())
		return
	}

	http.Handle(*metricsPath, promhttp.Handler())

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Named Process Exporter</title></head>
			<body>
			<h1>Named Process Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})
	server := &http.Server{Addr: *listenAddress}
	if err := web.ListenAndServe(server, *tlsConfigFile, logger); err != nil {
		log.Fatalf("Failed to start the server: %v", err)
		os.Exit(1)
	}
}
