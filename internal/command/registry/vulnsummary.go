package registry

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/superfly/flyctl/internal/command"
	"github.com/superfly/flyctl/internal/flag"
	"github.com/superfly/flyctl/internal/render"
	"github.com/superfly/flyctl/iostreams"
)

func newVulnSummary() *cobra.Command {
	const (
		usage = "vulnsummary <vulnid> ... [flags]"
		short = "Show a summary of possible vulnerabilities in registry images"
		long  = "Summarize possible vulnerabilities in registry images in an org, by app.\n" +
			"Limit scanning to a single app if specified. Limit scanning to images\n" +
			"used by running machines if specified. Limit reporting to\n" +
			"specific vulnerability IDs or severities if specified."
	)
	cmd := command.New(usage, short, long, runVulnSummary,
		command.RequireSession,
		command.LoadAppNameIfPresent,
	)

	cmd.Args = cobra.ArbitraryArgs
	flag.Add(
		cmd,
		flag.App(),
		flag.Org(),
		flag.Bool{
			Name:        "running",
			Shorthand:   "r",
			Description: "Only scan images for running machines",
		},
		flag.String{
			Name:        "severity",
			Shorthand:   "S",
			Description: fmt.Sprintf("Report only issues with a specific severity %v", allowedSeverities),
		},
	)

	return cmd
}

func runVulnSummary(ctx context.Context) error {
	var err error
	filter, err := argsGetVulnFilter(ctx)
	if err != nil {
		return err
	}

	imgs, err := argsGetImages(ctx)
	if err != nil {
		return err
	}

	// fetch all image scans.
	// TODO: spinner for long running fetches.
	ios := iostreams.FromContext(ctx)
	imageScan := map[string]*Scan{}
	token := ""
	tokenAppID := ""
	for _, img := range imgs {
		if _, ok := imageScan[img.Path]; ok {
			continue
		}

		if img.AppID != tokenAppID {
			tokenAppID = img.AppID
			token, err = makeScantronToken(ctx, img.OrgID, img.AppID)
			if err != nil {
				return err
			}
		}

		scan, err := getVulnScan(ctx, img.Path, token)
		if err != nil {
			errUnsupportedPath := ErrUnsupportedPath("")
			if errors.As(err, &errUnsupportedPath) {
				fmt.Fprintf(ios.Out, "Skipping %s (%s) from unsupported repository: %s\n", img.App, img.Mach, img.Path)
				continue
			}
			return fmt.Errorf("Getting vulnerability scan for %s (%s): %w", img.App, img.Mach, err)
		}
		imageScan[img.Path] = filterScan(scan, filter)
	}

	// calculate findings tables
	allVids := map[string]bool{}
	vidsByApp := map[string]map[string]bool{}
	appImgsScanned := map[string]bool{}
	for _, img := range imgs {
		scan := imageScan[img.Path]
		if scan == nil {
			continue
		}

		k := fmt.Sprintf("%s/%s", img.AppID, img.Path)
		if _, ok := appImgsScanned[k]; ok {
			continue
		}
		appImgsScanned[k] = true

		if _, ok := vidsByApp[img.App]; !ok {
			vidsByApp[img.App] = map[string]bool{}
		}
		appVids := vidsByApp[img.App]

		for _, res := range scan.Results {
			for _, vuln := range res.Vulnerabilities {
				vid := vuln.VulnerabilityID
				allVids[vid] = true
				appVids[vid] = true
			}
		}
	}

	// Show what is being scanned.
	lastOrg := ""
	lastApp := ""
	fmt.Fprintf(ios.Out, "Scanned images\n")
	for _, img := range imgs {
		scan := imageScan[img.Path]
		if img.Org != lastOrg {
			fmt.Fprintf(ios.Out, "Org: %s\n", img.Org)
			lastOrg = img.Org
		}
		if img.App != lastApp {
			fmt.Fprintf(ios.Out, "  App: %s\n", img.App)
			lastApp = img.App
		}
		if scan != nil {
			fmt.Fprintf(ios.Out, "    %s\t%s\n", img.Mach, img.Path)
		} else {
			fmt.Fprintf(ios.Out, "    %s\t%s [skipped]\n", img.Mach, img.Path)
		}
	}
	fmt.Fprintf(ios.Out, "\n")
	fmt.Fprintf(ios.Out, "To scan an image run: flyctl scan vulns -a <app> -i <imgpath>\n")
	fmt.Fprintf(ios.Out, "To download an SBOM run: flyctl scan sbom -a <app> -i <imgpath>\n")
	fmt.Fprintf(ios.Out, "\n")

	// Report checkmark table with columns of apps and rows of vulns.
	apps := lo.Keys(vidsByApp)
	slices.SortFunc(apps, strings.Compare)
	vids := lo.Keys(allVids)
	slices.SortFunc(vids, cmpVulnId)
	slices.Reverse(vids)

	var rows [][]string
	for _, vid := range vids {
		row := []string{vid}
		for _, app := range apps {
			check := lo.Ternary(vidsByApp[app][vid], "X", "-")
			row = append(row, check)
		}
		rows = append(rows, row)
	}
	cols := append([]string{""}, apps...)
	render.Table(ios.Out, "Vulnerabilities in Apps", rows, cols...)

	return nil
}