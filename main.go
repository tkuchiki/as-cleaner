package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/tkuchiki/aws-sdk-go-config"
	"github.com/tkuchiki/parsetime"
	"gopkg.in/alecthomas/kingpin.v2"
)

func newEC2DescribeImagesInput() *ec2.DescribeImagesInput {
	return &ec2.DescribeImagesInput{
		// DryRun: aws.Bool(false),
		// ExecutableUsers: []*string{aws.String("")},
		// Filters: []*ec2.Filter,
		// ImageIds: []*string{aws.String("")},
		Owners: []*string{aws.String("self")},
	}
}

func newEC2DescribeSnapshotsInput() *ec2.DescribeSnapshotsInput {
	return &ec2.DescribeSnapshotsInput{
		// DryRun: aws.Bool(false),
		// Filters: []*ec2.Filter,
		// MaxResults: aws.Int64(),
		// NextToken: aws.String(),
		// OwnerIds: []*string{aws.String("")},
		// RestorableByUserIds: []*string{aws.String("")},
		OwnerIds: []*string{aws.String("self")},
	}
}

func newEC2DescribeVolumesInput() *ec2.DescribeVolumesInput {
	return &ec2.DescribeVolumesInput{
	// DryRun: aws.Bool(false),
	// Filters: []*ec2.Filter,
	// MaxResults: aws.Int64(),
	// NextToken: aws.String(),
	// VolumeIds: []*string{aws.String("")},
	}
}

func newEC2DeleteSnapshotInput(id string, dryRun bool) *ec2.DeleteSnapshotInput {
	return &ec2.DeleteSnapshotInput{
		SnapshotId: aws.String(id),
		DryRun:     aws.Bool(dryRun),
	}
}

func getImageIDs(svc *ec2.EC2) (map[string]bool, error) {
	var amis map[string]bool
	resp, err := svc.DescribeImages(newEC2DescribeImagesInput())
	if err != nil {
		return amis, err
	}

	amis = make(map[string]bool)

	for _, ami := range resp.Images {
		amis[*ami.ImageId] = true
	}

	return amis, nil
}

func extractDescription(description string) string {
	re := regexp.MustCompile(`Created by CreateImage\((?:.+)\) for (.+) from (?:.+)`)
	group := re.FindStringSubmatch(description)

	if len(group) != 2 {
		return ""
	}

	return group[1]
}

func parseNameValues(val string) (string, []string) {
	nameValues := strings.SplitN(val, ",", 2)
	name := nameValues[0]
	values := nameValues[1]

	parsedName := strings.Split(name, "=")[1]
	parsedValues := strings.Split(strings.SplitN(values, "=", 2)[1], ",")

	return parsedName, parsedValues
}

func cmpTime(t time.Time, begin, end, timezone string) (bool, error) {
	var err error
	var p parsetime.ParseTime
	if timezone == "" {
		p, err = parsetime.NewParseTime()
	} else {
		p, err = parsetime.NewParseTime(timezone)
	}

	if err != nil {
		return false, err
	}

	t.In(p.GetLocation())

	var beginT, endT time.Time
	if begin != "" {
		beginT, err = p.Parse(begin)
		if err != nil {
			return false, err
		}
	}

	if end != "" {
		endT, err = p.Parse(end)
		if err != nil {
			return false, err
		}
	}

	if !beginT.IsZero() && endT.IsZero() {
		return beginT.Unix() <= t.Unix(), nil
	} else if beginT.IsZero() && !endT.IsZero() {
		return endT.Unix() >= t.Unix(), nil
	} else {
		return false, nil
	}

	return beginT.Unix() <= t.Unix() && endT.Unix() >= t.Unix(), nil
}

var (
	filters   = kingpin.Flag("filters", "filter tags (Name=xxx,Values=yyy,zzz Name=xxx,Values=yyy...)").Short('f').String()
	beginTime = kingpin.Flag("begin-time", "snapshot start time begin").String()
	endTime   = kingpin.Flag("end-time", "snapshot start time end").String()
	timezone  = kingpin.Flag("timezone", "timezone (default: local timezone)").Short('t').String()
	noDryRun  = kingpin.Flag("no-dry-run", "disable dry-run mode").Bool()
	rmVolume  = kingpin.Flag("rm-volume", "remove volume").Bool()
	accessKey = kingpin.Flag("access-key", "AWS Access Key ID").String()
	secretKey = kingpin.Flag("secret-key", "AWS Secret Access Key").String()
	profile   = kingpin.Flag("profile", "specific profile from your credential file").String()
	config    = kingpin.Flag("config", "AWS shared config file").String()
	credsPath = kingpin.Flag("credentials", "AWS shared credentials file").String()
	region    = kingpin.Flag("region", "AWS region").String()
)

func main() {
	kingpin.Version("0.1.0")
	kingpin.Parse()

	dryRun := !*noDryRun

	var awsConfig *aws.Config
	var err error
	awsConfig, err = awsconfig.NewConfig(awsconfig.Option{
		AccessKey:   *accessKey,
		SecretKey:   *secretKey,
		Profile:     *profile,
		Config:      *config,
		Credentials: *credsPath,
		Region:      *region,
	})
	if err != nil {
		log.Fatal(err)
	}

	svc := ec2.New(session.New(), awsConfig)

	resp, err := svc.DescribeSnapshots(newEC2DescribeSnapshotsInput())
	if err != nil {
		log.Fatal(err)
	}

	snapshots := make(map[string][]ec2.Snapshot)

	for _, s := range resp.Snapshots {
		ami := extractDescription(*s.Description)
		if ami == "" {
			continue
		}
		snapshots[ami] = append(snapshots[ami], *s)
	}

	var amis map[string]bool
	amis, err = getImageIDs(svc)
	if err != nil {
		log.Fatal(err)
	}

	splitedFilters := strings.Fields(*filters)

	matchedSnapshots := make(map[string][]ec2.Snapshot)

	for ami, ss := range snapshots {
		if _, ok := amis[ami]; ok {
			continue
		}

		for _, s := range ss {
			var matched bool
			for _, f := range splitedFilters {
				filterName, filterValues := parseNameValues(f)
				for _, tag := range s.Tags {
					if *tag.Key != filterName {
						continue
					}

				OrFilter:
					for _, filter := range filterValues {
						matched, err = regexp.MatchString(filter, *tag.Value)
						if err != nil {
							log.Fatal(err)
						}

						if matched {
							break OrFilter
						}
					}
				}
			}

			if len(splitedFilters) == 0 {
				matched = true
			}

			if !matched {
				continue
			}

			if *beginTime != "" || *endTime != "" {
				var isInRange bool
				isInRange, err = cmpTime(*s.StartTime, *beginTime, *endTime, *timezone)
				if err != nil {
					log.Fatal(err)
				}

				if !isInRange {
					continue
				}
			}

			matchedSnapshots[ami] = append(matchedSnapshots[ami], s)
		}
	}

	for ami, snapshots := range matchedSnapshots {
		var snapshotStr string
		for _, s := range snapshots {
			input := newEC2DeleteSnapshotInput(*s.SnapshotId, dryRun)
			_, err = svc.DeleteSnapshot(input)

			if reqErr, ok := err.(awserr.RequestFailure); ok {
				if reqErr.StatusCode() == 412 {
					if *rmVolume {
						snapshotStr = fmt.Sprintf("%s %s(%s)", snapshotStr, *s.SnapshotId, *s.VolumeId)
					} else {
						snapshotStr = fmt.Sprintf("%s %s", snapshotStr, *s.SnapshotId)
					}
				}
			} else {
				log.Fatal(reqErr)
			}
		}

		if dryRun {
			fmt.Println(fmt.Sprintf("dry run succeeded, %s %s", ami, strings.TrimLeft(snapshotStr, " ")))
		}
	}

}
