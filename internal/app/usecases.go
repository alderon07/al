package app

import (
	catalog "github.com/alderon07/al/internal/catalog"

	context "context"
	workflowplan "github.com/alderon07/al/internal/plan"
	providers "github.com/alderon07/al/internal/providers"
	workflowstate "github.com/alderon07/al/internal/state"
	os "os"
	time "time"
)

func (s *Services) ActiveShellAdapter() ShellAdapter { return s.activeShellAdapter() }
func (s *Services) AddAlias(name, command, description string) error {
	return s.addAlias(name, command, description)
}
func (s *Services) AddAliasWithMetadata(name, command, description string, metadata EntryMetadata) error {
	return s.addAliasWithMetadata(name, command, description, metadata)
}

func (s *Services) AddDefaultAliasesToFile(path string) (int, error) {
	return s.addDefaultAliasesToFile(path)
}
func (s *Services) AliasCommandMap(contents []byte) map[string]string {
	return aliasCommandMap(contents)
}
func (s *Services) AliasDisplayPath() string { return s.aliasDisplayPath() }

func (s *Services) BuildCatalogConflictResolutionPlan(id string, choices map[string]string) (workflowplan.OperationPlan, error) {
	return s.buildCatalogConflictResolutionPlan(id, choices)
}
func (s *Services) BuildCatalogSyncPullPlan() (workflowplan.OperationPlan, error) {
	return s.buildCatalogSyncPullPlan()
}

func (s *Services) BuildProfilePlan(action, name string) (workflowplan.OperationPlan, error) {
	return s.buildProfilePlan(action, name)
}

func (s *Services) CatalogInitRemoteSource(source string) bool {
	return catalogInitRemoteSource(source)
}

func (s *Services) CloneAndConfigureRepository(ctx context.Context, config AppConfig, connected providers.RepoProvider, repo providers.RemoteRepo) (RepositoryConfiguration, error) {
	return s.cloneAndConfigureRepository(ctx, config, connected, repo)
}
func (s *Services) CompareAliasFiles(local, remote []byte) (localOnly, remoteOnly []string, conflicts []AliasConflict) {
	return compareAliasFiles(local, remote)
}
func (s *Services) ConfigureRepository(path string) error { return s.configureRepository(path) }
func (s *Services) ContentHash(contents []byte) string    { return contentHash(contents) }

func (s *Services) CurrentContextRanking() (ContextRanking, error) {
	return s.currentContextRanking()
}

func (s *Services) DangerousCommandReasons(command string) []string {
	return dangerousCommandReasons(command)
}
func (s *Services) DecodeSyncCatalog(data []byte) (catalog.Catalog, error) {
	return decodeSyncCatalog(data)
}

func (s *Services) DeleteAlias(name string) error { return s.deleteAlias(name) }
func (s *Services) DiscoverRemoteCatalog(ctx context.Context, config AppConfig, source, path string) (RemoteCatalogPreview, error) {
	return s.discoverRemoteCatalog(ctx, config, source, path)
}

func (s *Services) DoctorChecks() []DoctorCheck { return s.doctorChecks() }
func (s *Services) EditAliasWithMetadata(originalName, name, command, description string, metadata EntryMetadata) error {
	return s.editAliasWithMetadata(originalName, name, command, description, metadata)
}
func (s *Services) EncodeCatalogJSON(value any) ([]byte, error) { return encodeCatalogJSON(value) }

func (s *Services) EnsureWatchProcess() error { return s.ensureWatchProcess() }
func (s *Services) Entries() ([]Alias, error) { return s.loadAliases() }

func (s *Services) HasUntimestampedUsage(events []UsageEvent) bool {
	return hasUntimestampedUsage(events)
}
func (s *Services) HashBytes(contents []byte) string { return hashBytes(contents) }

func (s *Services) HistorySuggestions(aliases []Alias, counts map[string]int) []HistorySuggestion {
	return historySuggestions(aliases, counts)
}

func (s *Services) InspectCatalogSync() error                   { return s.inspectCatalogSync() }
func (s *Services) InspectWorkflowStatus() workflowstate.Report { return s.inspectWorkflowStatus() }
func (s *Services) InteractiveInput(input *os.File) bool        { return interactiveInput(input) }
func (s *Services) InterruptContext() (context.Context, context.CancelFunc) {
	return interruptContext()
}
func (s *Services) IsDangerousCommand(command string) bool { return isDangerousCommand(command) }
func (s *Services) ListRemoteRepositories(ctx context.Context, config AppConfig, only string) ([]providers.RemoteRepo, []string) {
	return s.listRemoteRepositories(ctx, config, only)
}

func (s *Services) LoadCatalogConflictResolution(id string) (CatalogConflictResolution, error) {
	return s.loadCatalogConflictResolution(id)
}

func (s *Services) LoadHistoryCounts() map[string]int          { return s.loadHistoryCounts() }
func (s *Services) LoadShellEntry(name string) (string, error) { return s.loadShellEntry(name) }
func (s *Services) LoadStatsData() (StatsData, error)          { return s.loadStatsData() }

func (s *Services) LoadTheme() (Theme, error) { return s.loadTheme() }

func (s *Services) MarkTourSeen() error { return s.markTourSeen() }

func (s *Services) PrepareCatalogPush() (CatalogPushPreview, error) { return s.prepareCatalogPush() }

func (s *Services) PullRepository() (string, error) { return s.pullRepository() }

func (s *Services) RankedStatsRows(data StatsData, period string, now time.Time) ([]StatsRow, error) {
	return rankedStatsRows(data, period, now)
}

func (s *Services) RecordAliasUse(name string) error { return s.recordAliasUse(name) }

func (s *Services) RemoveSetup(shellName string) (SetupRemovalResult, error) {
	return s.performRemoveSetup(shellName)
}

func (s *Services) RequestedShellAdapter(name string) (ShellAdapter, error) {
	return s.requestedShellAdapter(name)
}
func (s *Services) RestoreReviewedRevision(request RevisionRestoreRequest) error {
	return s.restoreReviewedRevision(request)
}
func (s *Services) RestoreRevision(id string) (Revision, error) { return s.restoreNamedRevision(id) }
func (s *Services) RevisionList() ([]Revision, error)           { return s.loadRevisionList() }
func (s *Services) RevisionPreview(id string) (RevisionPreview, error) {
	return s.loadRevisionPreview(id)
}

func (s *Services) RunWatch(daemon bool) error  { return s.runWatch(daemon) }
func (s *Services) SaveTheme(theme Theme) error { return s.saveTheme(theme) }

func (s *Services) Setup(request SetupRequest) (SetupResult, error) { return s.performSetup(request) }
func (s *Services) ShellAdapter(name string) (ShellAdapter, error)  { return s.shellAdapter(name) }

func (s *Services) StatsPeriodSince(period string, now time.Time) (time.Time, error) {
	return statsPeriodSince(period, now)
}

func (s *Services) SyncRepository(push bool) (string, error) { return s.syncRepository(push) }

func (s *Services) TourShouldShow() bool { return s.tourShouldShow() }

func (s *Services) UpdateDescriptions() (DescriptionUpdate, error) { return s.describeEntries() }
func (s *Services) WatchRunningExecutable() ExecutableWatch        { return s.watchRunningExecutable() }
