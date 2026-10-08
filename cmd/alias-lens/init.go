//go:build !windows

package main

import (
	"github.com/alderon07/al/internal/app"

	workflowplan "github.com/alderon07/al/internal/plan"

	"fmt"
)

func init() { catalogInitImplementation = runSupportedCatalogInit }

func runSupportedCatalogInit(options app.CatalogInitOptions) error {
	if applicationServices().CatalogInitRemoteSource(options.Source) {
		return runRemoteCatalogInit(options)
	}
	record, value, source, err := applicationServices().ObserveLocalCatalogInit(options)
	if err != nil {
		return err
	}
	adapter, err := applicationServices().RequestedShellAdapter(options.Shell)
	if err != nil {
		return err
	}
	options.ReviewedSourceSHA = applicationServices().HashBytes(source)
	options.ReviewedHEAD = record.HEAD
	options.ReviewedBlob = record.Blob
	decisions := app.CatalogLifecycleDecisions{ConfirmedAt: applicationServices().LifecycleTimestamp(), StartupPaths: append([]string{}, options.StartupPaths...)}
	if catalogReviewTerminal() && !options.Apply {
		decisions, err = guidedCatalogLifecycleReview(value, adapter.Name(), true)
		if err != nil {
			return err
		}
		decisions.StartupPaths = append([]string{}, options.StartupPaths...)
	}
	preview, err := applicationServices().BuildLocalCatalogInitPlan(options, decisions)
	if err != nil {
		return err
	}
	fmt.Fprint(catalogWorkflowStdout, workflowplan.RenderPlain(preview))
	if !options.Apply {
		if !catalogReviewTerminal() {
			fmt.Fprintln(catalogWorkflowStdout, "Enter al init SOURCE --apply after reviewing this plan.")
			return nil
		}
		confirmed, err := confirmCatalogReview(newCatalogReviewReader(), "Apply this catalog enrollment and installation?")
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}
	if err := applicationServices().ApplyLocalCatalogInit(preview, options, decisions); err != nil {
		return err
	}
	fmt.Fprintln(catalogWorkflowStdout, "Catalog enrolled and installed. Start a new "+adapter.DisplayName()+" shell.")
	return nil
}

func runRemoteCatalogInit(options app.CatalogInitOptions) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	ctx, cancel := applicationServices().InterruptContext()
	defer cancel()
	remote, err := applicationServices().DiscoverRemoteCatalog(ctx, config, options.Source, options.CatalogPath)
	if err != nil {
		return err
	}
	value, err := applicationServices().DecodeSyncCatalog(remote.Bytes)
	if err != nil {
		return err
	}
	adapter, err := applicationServices().RequestedShellAdapter(options.Shell)
	if err != nil {
		return err
	}
	decisions := app.CatalogLifecycleDecisions{ConfirmedAt: applicationServices().LifecycleTimestamp(), StartupPaths: append([]string{}, options.StartupPaths...)}
	if catalogReviewTerminal() && !options.Apply {
		decisions, err = guidedCatalogLifecycleReview(value, adapter.Name(), true)
		if err != nil {
			return err
		}
		decisions.StartupPaths = append([]string{}, options.StartupPaths...)
	}
	preview, err := applicationServices().BuildRemoteCatalogInitPlan(options, remote, decisions, "")
	if err != nil {
		return err
	}
	fmt.Fprint(catalogWorkflowStdout, workflowplan.RenderPlain(preview))
	if !options.Apply {
		if !catalogReviewTerminal() {
			fmt.Fprintln(catalogWorkflowStdout, "Enter al init SOURCE --apply after reviewing this immutable catalog plan.")
			return nil
		}
		confirmed, err := confirmCatalogReview(newCatalogReviewReader(), "Apply this pinned remote catalog enrollment and installation?")
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}
	if err := applicationServices().ApplyRemoteCatalogInit(ctx, config, options, remote, decisions, preview); err != nil {
		return err
	}
	fmt.Fprintln(catalogWorkflowStdout, "Immutable catalog enrolled and installed. Start a new "+adapter.DisplayName()+" shell.")
	return nil
}
