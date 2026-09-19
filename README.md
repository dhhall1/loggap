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

A file argument ending in `.gz` is decompressed transparently:

```
loggap service.log.gz
```

(This only applies to a named file - gzip data piped in over stdin isn't
detected, since there's no `.gz` extension to key off of.)

### Flags

- `--min-gap DURATION` - smallest gap worth reporting (default `30s`).
  Accepts anything `time.ParseDuration` does: `10s`, `2m`, `1h30m`.
- `--json` - print the report as JSON instead of plain text.
- `--format LAYOUT` - a Go reference layout (the `Mon Jan 2 15:04:05 2006`
  style, built around the reference time `2006-01-02T15:04:05Z07:00`) for
  timestamps that don't match any built-in format. Tried at the start of
  each line only after all built-in formats have failed to match, so it
  never overrides them. If the layout has no year (like syslog), `loggap`
  assumes the current year and corrects for a log that rolled over a year
  boundary, the same way it does for the built-in syslog format.
- `--since TIME` / `--until TIME` - ignore lines timestamped outside this
  window. Accepts RFC3339 (`2024-03-01T09:00:00Z`), the same without a zone
  (`2024-03-01T09:00:00`, `2024-03-01 09:00:00`), or a bare date
  (`2024-03-01`, midnight UTC). Lines outside the window are skipped the
  same way lines with no recognized timestamp are - they don't break the
  gap calculation on either side of the window, they're just excluded from
  it. Useful for narrowing a big log down to the incident window before
  looking for gaps in it.

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
  ],
  "histogram": [
    {
      "min_seconds": 480,
      "count": 1
    }
  ]
}
```

`histogram` buckets gap sizes into powers of two starting at `--min-gap`
(30s, 60s, 120s, ... in the example above), so the shape of the
distribution is visible at a glance regardless of whether the gaps in a
log run from seconds to days. Buckets with no gaps in them are omitted.
The last bucket present has no `max_seconds` - it's unbounded above.

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

Lines that don't match any of these, and don't match `--format` when it's
given, are counted but otherwise ignored; they don't break the gap
calculation, they're just skipped.

For example, a log written as `2024.03.01-09:00:00 ...` needs:

```
loggap --format "2006.01.02-15:04:05" service.log
```

## Install

```
go install github.com/dhhall1/loggap@latest
```

Or clone and `go build`.

## License

MIT, see [LICENSE](LICENSE).
