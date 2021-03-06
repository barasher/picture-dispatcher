package internal

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/barasher/go-exiftool"
	"github.com/rs/zerolog/log"
)

const (
	fileLogField = "file"
)

var defaultThreadCount = runtime.NumCPU()
var defaultOutputDateFormat = "2006_01"
var errNoDateFound = fmt.Errorf("No data found")

type DateDispatcher struct {
	threadCount      int
	outputDateFormat string
	dateFields       map[string]string
	exiftoolPath     string
}

func OptThreadCount(size int) func(*DateDispatcher) error {
	return func(c *DateDispatcher) error {
		c.threadCount = size
		return nil
	}
}

func OptDateFields(fields map[string]string) func(*DateDispatcher) error {
	return func(c *DateDispatcher) error {
		for f, p := range fields {
			c.dateFields[f] = p
		}
		return nil
	}
}

func OptDateOutputFormat(pattern string) func(*DateDispatcher) error {
	return func(c *DateDispatcher) error {
		c.outputDateFormat = pattern
		return nil
	}
}

func OptExiftoolPath(path string) func(*DateDispatcher) error {
	return func(c *DateDispatcher) error {
		c.exiftoolPath = path
		return nil
	}
}

func NewDateDispatcher(classOpts ...func(*DateDispatcher) error) (*DateDispatcher, error) {
	c := DateDispatcher{
		threadCount:      runtime.NumCPU(),
		dateFields:       make(map[string]string),
		outputDateFormat: defaultOutputDateFormat,
	}
	for _, opt := range classOpts {
		if err := opt(&c); err != nil {
			return nil, fmt.Errorf("error when configuring date dispatcher: %v", err)
		}
	}
	return &c, nil
}

func (dd *DateDispatcher) Dispatch(inputFolder string, outputFolder string) error {
	ctx, cancel := context.WithCancel(context.Background())
	fileChan := make(chan string, dd.threadCount)
	actionChan := make(chan moveAction, dd.threadCount)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() { // list files
		dd.listFiles(ctx, cancel, inputFolder, fileChan)
		defer wg.Done()
	}()

	go func() {
		dd.getMoveActions(ctx, cancel, fileChan, actionChan)
		defer wg.Done()
	}()

	go func() {
		dd.moveFiles(ctx, cancel, outputFolder, actionChan)
		defer wg.Done()
	}()

	wg.Wait()
	return nil
}

func (dd *DateDispatcher) listFiles(ctx context.Context, cancel context.CancelFunc, inputFolder string, filesChan chan string) {
	defer close(filesChan)
	fileCount := 0
	var err2 error

	err2 = filepath.Walk(inputFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error when browsing file %v: %v", path, err)
		}
		if !info.IsDir() {
			select {
			case <-ctx.Done():
				return nil
			default:
				filesChan <- path
				fileCount++
				log.Debug().Msgf("New file to extract: %v", path)
			}
		}
		return nil
	})

	if err2 != nil {
		cancel()
		log.Error().Msgf("%v", err2)
	}
	log.Info().Msgf("%v file(s) found", fileCount)
}

type moveAction struct {
	from string
	to   string
}

func (dd *DateDispatcher) getMoveActions(ctx context.Context, cancel context.CancelFunc, filesChan chan string, actionChan chan moveAction) error {
	wg := sync.WaitGroup{}
	wg.Add(dd.threadCount)
	for i := 0; i < dd.threadCount; i++ {
		go func(thId int) {
			defer wg.Done()
			l := log.With().Int("threadId", thId).Logger()

			opts := []func(*exiftool.Exiftool) error{}
			if dd.exiftoolPath != "" {
				opts = append(opts, exiftool.SetExiftoolBinaryPath(dd.exiftoolPath))
			}
			exif, err := exiftool.NewExiftool(opts...)
			if err != nil {
				l.Error().Msgf("error while initializing go-exiftool: %v", err)
				return
			}
			defer exif.Close()

			for {
				select {
				case <-ctx.Done():
					return
				case file, found := <-filesChan:
					if !found {
						return
					}

					fm := exif.ExtractMetadata(file)
					if fm[0].Err != nil {
						l.Warn().Str(fileLogField, file).Msgf("error while extracting metadata: %v", fm[0].Err)
						continue
					}

					if d, err := dd.guessDate(fm[0]); err != nil {
						if err != errNoDateFound {
							l.Error().Str(fileLogField, file).Msgf("error while generating moveAction %v", err)
						}
					} else {
						actionChan <- moveAction{
							from: file,
							to:   d.Format(dd.outputDateFormat),
						}
					}
				}
			}

		}(i)
	}
	wg.Wait()
	close(actionChan)
	return nil
}

func (dd *DateDispatcher) guessDate(fm exiftool.FileMetadata) (time.Time, error) {
	for field, pattern := range dd.dateFields {
		if val, found := fm.Fields[field]; found {
			t, err := time.Parse(pattern, val.(string))
			if err != nil {
				return time.Time{}, fmt.Errorf("error when parsing date %v: %v", val.(string), err)
			}
			return t, nil
		}
	}
	return time.Time{}, errNoDateFound
}

func (dd *DateDispatcher) moveFiles(ctx context.Context, cancel context.CancelFunc, outputFolder string, actionChan chan moveAction) {
	moveCount := 0
	dirs := make(map[string]bool)
	for ma := range actionChan {
		l := log.With().Str(fileLogField, ma.from).Logger()
		select {
		case <-ctx.Done():
			log.Info().Msgf("moveFiles canceled")
		default:
			if _, found := dirs[ma.to]; !found {
				if err := os.MkdirAll(filepath.Join(outputFolder, ma.to), 0777); err != nil {
					l.Error().Msgf("error when creating output folder: %v", err)
					continue
				}
				dirs[ma.to] = true
			}
			_, f := filepath.Split(ma.from)
			to := filepath.Join(outputFolder, ma.to, f)
			l.Debug().Msgf("Moving to %v", to)
			if err := move(ma.from, to); err != nil {
				l.Error().Msgf("error when moving %v: %v", to, err)
			} else {
				moveCount++
			}
		}
	}
	log.Info().Msgf("%v moved file(s)", moveCount)
}

func copy(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.Create(to)
	if err != nil {
		return err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return err
}

func move(from, to string) error {
	if err := copy(from, to); err != nil {
		return err
	}
	return os.Remove(from)
}
