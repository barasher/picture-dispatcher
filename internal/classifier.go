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

type Classifier struct {
	threadCount      int
	outputDateFormat string
	dateFields       map[string]string
}

func OptThreadCount(size int) func(*Classifier) error {
	return func(c *Classifier) error {
		c.threadCount = size
		return nil
	}
}

func OptDateFields(fields map[string]string) func(*Classifier) error {
	return func(c *Classifier) error {
		for f, p := range fields {
			c.dateFields[f] = p
		}
		return nil
	}
}

func NewClassifier(classOpts ...func(*Classifier) error) (*Classifier, error) {
	c := Classifier{
		threadCount:      runtime.NumCPU(),
		dateFields:       make(map[string]string),
		outputDateFormat: defaultOutputDateFormat,
	}
	for _, opt := range classOpts {
		if err := opt(&c); err != nil {
			return nil, fmt.Errorf("error when configuring classifier: %v", err)
		}
	}
	return &c, nil
}

func (cl *Classifier) Classify(inputFolder string, outputFolder string) error {
	ctx, cancel := context.WithCancel(context.Background())
	fileChan := make(chan string, cl.threadCount)
	actionChan := make(chan moveAction, cl.threadCount)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() { // list files
		cl.listFiles(ctx, cancel, inputFolder, fileChan)
		defer wg.Done()
	}()

	go func() {
		cl.getMoveActions(ctx, cancel, fileChan, actionChan)
		defer wg.Done()
	}()

	wg.Wait()
	return nil
}

func (cl *Classifier) listFiles(ctx context.Context, cancel context.CancelFunc, inputFolder string, filesChan chan string) {
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
			case filesChan <- path:
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

func (cl *Classifier) getMoveActions(ctx context.Context, cancel context.CancelFunc, filesChan chan string, actionChan chan moveAction) error {
	wg := sync.WaitGroup{}
	wg.Add(cl.threadCount)
	for i := 0; i < cl.threadCount; i++ {
		go func(thId int) {
			defer wg.Done()
			l := log.With().Int("threadId", thId).Logger()

			exif, err := exiftool.NewExiftool()
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

					if d, err := cl.guessDate(fm[0]); err != nil {
						if err != errNoDateFound {
							l.Error().Str(fileLogField, file).Msgf("error while generating moveAction %v", err)
						}
					} else {
						actionChan <- moveAction{
							from: file,
							to:   d.Format(cl.outputDateFormat),
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

func (cl *Classifier) guessDate(fm exiftool.FileMetadata) (time.Time, error) {
	for field, pattern := range cl.dateFields {
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

func (cl *Classifier) moveFiles(ctx context.Context, cancel context.CancelFunc, outputFolder string, actionChan chan moveAction) {
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
