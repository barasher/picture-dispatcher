# Picture-dispatcher


[![Build Status](https://github.com/barasher/picture-dispatcher/workflows/DispatcherCI/badge.svg)](https://github.com/barasher/picture-dispatcher/actions)[![Build Status](https://github.com/barasher/picture-dispatcher/workflows/DispatcherWindowsCI/badge.svg)](https://github.com/barasher/picture-dispatcher/actions)
[![go report card](https://goreportcard.com/badge/github.com/barasher/picture-dispatcher "go report card")](https://goreportcard.com/report/github.com/barasher/picture-dispatcher)

## Presentation

**Picture-dispatcher** is a CLI tool designed to :
- drop Apple live pictures
- dispatch multimedia files (pictures, movies) by date

## Execution

```
$ ./dispatcher -h
Usage of file-dispatcher:
  -c string
    	Configuration file
  -d string
    	Destination folder
  -s string
    	Source folder
```

Dispatch files contained in `/path/containing/pictures` in `/path/to/store/dispatched` 

`$ ./dispatcher -c dispatcher.json -s /path/containing/pictures -d /path/to/store/dispatched` dispatchs files contained in `/path/containing/pictures` in `/path/to/store/dispatched`.

If `-d` is not provided, a new `out` folder will be created in de "source" folder. `$ ./dispatcher -c dispatcher.json -s /path/containing/pictures` will dispatch files contained in `/path/containing/pictures` in `/path/containing/pictures/out`.

## Configuration

```json
{
    "loggingLevel":"info",
    "threadCount":2,
    "dateFields": [
        { "field":"CreateDate", "pattern":"2006:01:02 15:04:05" },
        { "field":"Media Create Date", "pattern":"2006:01:02 15:04:05" }
    ],
    "outputDateFormat":"2006_01",
    "exiftoolPath":"/path/to/exiftool"
}
```

- **loggingLevel** : (optional) logging level (debug, info, warn, error, fatal, panic)
- **threadCount** : (optional, default : max proc) how many goroutines are spawned to extract date
- **dateFields** : exiftool tags that have to be considered as valid date for dispatching
  - **dateFields.field** : exiftool tag key
  - **dateFields.pattern** : date pattern, based on golang specifications (https://golang.org/pkg/time/#Time.Format)
- **outputDateFormat** : date pattern for the output folders, based on golang specifications (https://golang.org/pkg/time/#Time.Format)
- **exiftoolPath** : (optional) path to `exiftool` binary if not in `$PATH`