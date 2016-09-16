# as-cleaner
Delete Amazon EBS Snapshot for AMI

# Installation

Download from  https://github.com/tkuchiki/as-cleaner/releases

# Usage

```
$ ./as-cleaner --help
usage: as-cleaner [<flags>]

Flags:
      --help                     Show context-sensitive help (also try --help-long and --help-man).
  -f, --filters=FILTERS          filter tags (Name=xxx,Values=yyy,zzz Name=xxx,Values=yyy...)
      --begin-time=BEGIN-TIME    snapshot start time begin
      --end-time=END-TIME        snapshot start time end
  -t, --timezone=TIMEZONE        timezone (default: local timezone)
      --no-dry-run               disable dry-run mode
      --rm-volume                remove volume
      --access-key=ACCESS-KEY    AWS Access Key ID
      --secret-key=SECRET-KEY    AWS Secret Access Key
      --profile=PROFILE          specific profile from your credential file
      --config=CONFIG            AWS shared config file
      --credentials=CREDENTIALS  AWS shared credentials file
      --region=REGION            AWS region
      --version                  Show application version.
```

# Examples

## Dry run

```
$ ./as-cleaner
dry run succeeded, ami-xxxxxxxx snap-xxxxxxxx
dry run succeeded, ami-yyyyyyyy snap-yyyyyyyy snap-yyyyyyyy
```

```
$ ./as-cleaner --rm-volume
dry run succeeded, ami-xxxxxxxx snap-xxxxxxxx(vol-xxxxxxxx)
dry run succeeded, ami-yyyyyyyy snap-yyyyyyyy(vol-yyyyyyyy) snap-zzzzzzzz(vol-zzzzzzzz)
```

## Run

```
$ ./as-cleaner --no-dry-run
deleted ami-xxxxxxxx snap-xxxxxxxx
deleted ami-yyyyyyyy snap-yyyyyyyy snap-yyyyyyyy
```
