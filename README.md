# loggap

Find gaps in time between consecutive timestamped lines in a log file.

A service that hangs, deadlocks, or gets starved of CPU right before it
crashes will often just stop logging for a while - no error, no stack
trace, just silence. In a file with tens of thousands of lines that
silence is easy to scroll right past. `loggap` reads a log file line by
line, pulls a timestamp out of each line it can, and reports every stretch
of time where nothing was logged for longer than a threshold you set.

## Usage

```
loggap [flags] [file]
```

If no file is given, `loggap` reads from stdin, so it works fine at the
end of a pipe:

```
journalctl -u myservice --since today | loggap --min-gap 1m
```

### Flags

- `--min-gap DURATION` - smallest gap worth reporting (default `30s`).
  Accepts anything `time.ParseDuration` does: `10s`, `2m`, `1h30m`.
- `--json` - print the report as JSON instead of plain text.

### Example

Given `service.log`:

```
2024-03-01T09:00:00Z INFO starting up
2024-03-01T09:00:01Z INFO listening on :8080
2024-03-01T09:00:02Z INFO handled request /health
2024-03-01T09:14:57Z WARN slow query took 3.2s
2024-03-01T09:14:58Z ERROR panic: nil pointer dereference
```

```
$ loggap --min-gap 30s service.log
line 3 -> 4: 14m55s silence (2024-03-01T09:00:02Z -> 2024-03-01T09:14:57Z)

1 gap(s) >= 30s, 5/5 lines had a recognized timestamp
```

The same run with `--json`:

```
$ loggap --min-gap 30s --json service.log
{
  "file": "service.log",
  "min_gap_seconds": 30,
  "lines_scanned": 5,
  "lines_with_timestamp": 5,
  "gaps": [
    {
      "from_line": 3,
      "to_line": 4,
      "from": "2024-03-01T09:00:02Z",
      "to": "2024-03-01T09:14:57Z",
      "seconds": 895
    }
  ]
}
```

The JSON mode is meant for piping into `jq` or feeding to another tool -
the text mode is meant for reading at a terminal while chasing down an
incident.

## Timestamp formats

`loggap` currently recognizes, at the start of a line:

- RFC 3339 (`2024-03-01T09:00:00Z`, with or without fractional seconds and
  with either `Z` or a numeric offset)
- Apache/nginx combined log format (`02/Jan/2024:15:04:05 -0700`)
- Classic syslog (`Jan  2 15:04:05`) - since this format has no year,
  `loggap` assumes the current year, correcting for the case where the log
  rolled over a year boundary

Lines that don't match any of these are counted but otherwise ignored;
they don't break the gap calculation, they're just skipped.

## Install

```
go install github.com/dhhall1/loggap@latest
```

Or clone and `go build`.

## License

MIT, see [LICENSE](LICENSE).
