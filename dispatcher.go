package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/barasher/picture-dispatcher/internal"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	retOk          int = 0
	retConfFailure int = 1
	retExecFailure int = 2

	defaultLoggingLevel     string = "info"
	defaultOutputDateFormat string = "2006_01"
)

type dateField struct {
	Field   string `json:"field"`
	Pattern string `json:"pattern"`
}

type dispatcherConf struct {
	LoggingLevel     string      `json:"loggingLevel"`
	ThreadCount      int         `json:"threadCount"`
	DateFields       []dateField `json:"dateFields"`
	OutputDateFormat string      `json:"outputDateFormat"`
	ExiftoolPath     string      `json:"exiftoolPath"`
}

func loadConf(confFile string) (dispatcherConf, error) {
	c := dispatcherConf{}

	r, err := os.Open(confFile)
	if err != nil {
		return c, fmt.Errorf("Error while opening configuration file %v :%v", confFile, err)
	}
	err = json.NewDecoder(r).Decode(&c)
	if err != nil {
		return c, fmt.Errorf("Error while unmarshaling configuration file %v :%v", confFile, err)
	}

	if c.ThreadCount < 1 {
		c.ThreadCount = 0
		log.Warn().Msgf("No thread count specified (or 0), will fallback to default value")
	}
	if c.LoggingLevel == "" {
		c.LoggingLevel = defaultLoggingLevel
		log.Warn().Msgf("No logging level specified, using default (%v)", c.LoggingLevel)
	}
	if c.OutputDateFormat == "" {
		c.OutputDateFormat = defaultOutputDateFormat
		log.Warn().Msgf("No output date format specified, using default (%v)", c.OutputDateFormat)
	}

	if len(c.DateFields) == 0 {
		return c, fmt.Errorf("No date fields specified in the configuration file")
	}

	return c, nil
}

func setLoggingLevel(lvl string) error {
	if lvl != "" {
		lvl, err := zerolog.ParseLevel(lvl)
		if err != nil {
			return fmt.Errorf("error while setting logging level (%v): %w", lvl, err)
		}
		zerolog.SetGlobalLevel(lvl)
		log.Debug().Msgf("Logging level: %v", lvl)
	}
	return nil
}

func main() {
	os.Exit(doMain(os.Args))
}

func doMain(args []string) int {
	cmd := flag.NewFlagSet("Classifier", flag.ContinueOnError)
	from := cmd.String("s", "", "Source folder")
	to := cmd.String("d", "", "Destination folder")
	confFile := cmd.String("c", "", "Configuration file")

	err := cmd.Parse(args[1:])
	if err != nil {
		if err != flag.ErrHelp {
			log.Error().Msgf("error while parsing command line arguments: %v", err)
		}
		return retConfFailure
	}

	if *confFile == "" {
		log.Error().Msgf("No configuration file provided (-c)")
		return retConfFailure
	}
	conf, err := loadConf(*confFile)
	if err != nil {
		log.Error().Msgf("Error during configuration file validation: %v", err)
		return retConfFailure
	}

	if err = setLoggingLevel(conf.LoggingLevel); err != nil {
		log.Error().Msgf("error while specifying logging level: %v", err)
		return retConfFailure
	}

	if *from == "" {
		log.Error().Msgf("No source provided (-s)")
		return retConfFailure
	}

	if *to == "" {
		*to = filepath.Join(*from, "out")
		log.Info().Msgf("No destination provided (-s), defaults to %v", *to)
		if err = os.Mkdir(*to, 0777); err != nil {
			log.Error().Msgf("error while creating output folder (%v): %v", *to, err)
			return retConfFailure
		}
	}

	if err = internal.RemoveLiveVideos(*from); err != nil {
		log.Error().Msgf("error while removing live videos: %v", err)
		return retExecFailure
	}

	ddOpts := []func(*internal.DateDispatcher) error{}
	ddOpts = append(ddOpts, internal.OptDateOutputFormat(conf.OutputDateFormat))
	if conf.ThreadCount > 0 {
		ddOpts = append(ddOpts, internal.OptThreadCount(conf.ThreadCount))
	}
	if conf.ExiftoolPath != "" {
		ddOpts = append(ddOpts, internal.OptExiftoolPath(conf.ExiftoolPath))
	}
	dFs := map[string]string{}
	for _, v := range conf.DateFields {
		dFs[v.Field] = v.Pattern
	}
	if len(dFs) > 0 {
		ddOpts = append(ddOpts, internal.OptDateFields(dFs))
	}

	dd, err := internal.NewDateDispatcher(ddOpts...)
	if err != nil {
		log.Error().Msgf("error while initializing date dispatcher: %v", err)
		return retExecFailure
	}

	if err = dd.Dispatch(*from, *to); err != nil {
		log.Error().Msgf("error while dispatching date: %v", err)
		return retExecFailure
	}

	return retOk
}
