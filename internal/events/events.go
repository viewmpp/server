package events

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

type Name string

const (
	UploadOK          Name = "upload_ok"
	UploadRejected    Name = "upload_rejected"
	UploadTooLarge    Name = "upload_too_large"
	UploadInterrupted Name = "upload_interrupted"

	ExportXLSX     Name = "export_xlsx"
	ExportSaved    Name = "export_saved"
	ExportTooLarge Name = "export_too_large"
	SaveTooLarge   Name = "save_too_large"
	SaveLimit      Name = "save_limit"

	ShareCreated      Name = "share_created"
	ShareOpen         Name = "share_open"
	ShareLimit        Name = "share_limit"
	SharePageFromLink Name = "share_page_from_link"

	ExampleOpen Name = "example_open"

	CompareRun       Name = "compare_run"
	CompareExport    Name = "compare_export"
	CompareLocked    Name = "compare_export_locked"
	CompareOpen      Name = "compare_open"
	CompareFrom      Name = "compare_from"
	UploadForCompare Name = "upload_for_compare"

	VersionUploaded    Name = "version_uploaded"
	VersionChangesOpen Name = "version_changes_open"
	UploadForVersion   Name = "upload_for_version"

	WishPDF        Name = "wish_pdf"
	WishXLSXReport Name = "wish_xlsx_report"
	ExportAsked    Name = "export_asked"
	ExportPurpose  Name = "export_purpose"
)

const OtherPage = "other"

func UploadFrom(page string) Name {
	return Name("upload_from:" + page)
}

var (
	perViewer    = []Name{WishPDF, WishXLSXReport, CompareOpen, CompareLocked, VersionChangesOpen}
	viewers      = []string{"owner", "shared", "file", "example"}
	purposes     = []string{"share", "import", "analyze", "print", "other"}
	compareFroms = []string{"file", "project"}
)

var fromViewer = allowedFromViewer()

func allowedFromViewer() map[string]Name {
	allowed := map[string]Name{string(ExportAsked): ExportAsked}

	for _, event := range perViewer {
		for _, viewer := range viewers {
			name := event + Name(":"+viewer)
			allowed[string(name)] = name
		}
	}

	for _, purpose := range purposes {
		name := ExportPurpose + Name(":"+purpose)
		allowed[string(name)] = name
	}

	for _, from := range compareFroms {
		name := CompareFrom + Name(":"+from)
		allowed[string(name)] = name
	}

	return allowed
}

func FromViewer(raw string) (Name, bool) {
	name, ok := fromViewer[raw]
	return name, ok
}

const (
	FlushInterval = time.Minute
	flushTimeout  = 5 * time.Second
)

type store interface {
	Add(ctx context.Context, day string, name Name, total int64) error
}

type key struct {
	day  string
	name Name
}

type Recorder struct {
	mu      sync.Mutex
	pending map[key]int64
	store   store
	now     func() time.Time
	logger  *slog.Logger
}

func NewRecorder(store store, logger *slog.Logger) *Recorder {
	return &Recorder{
		pending: make(map[key]int64),
		store:   store,
		now:     time.Now,
		logger:  logger,
	}
}

func (r *Recorder) Count(name Name) {
	k := key{day: r.now().UTC().Format(time.DateOnly), name: name}

	r.mu.Lock()
	r.pending[k]++
	r.mu.Unlock()
}

func (r *Recorder) Flush(ctx context.Context) error {
	r.mu.Lock()
	batch := r.pending
	r.pending = make(map[key]int64)
	r.mu.Unlock()

	failed := make(map[key]int64)
	var errs []error

	for k, total := range batch {
		if err := r.store.Add(ctx, k.day, k.name, total); err != nil {
			failed[k] = total
			errs = append(errs, err)
		}
	}

	if len(failed) == 0 {
		return nil
	}

	r.mu.Lock()
	for k, total := range failed {
		r.pending[k] += total
	}
	r.mu.Unlock()

	return errors.Join(errs...)
}

func (r *Recorder) Start(stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(FlushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				r.flushLogged("event counts kept in memory, will retry")
			}
		}
	}()
}

func (r *Recorder) Close() {
	r.flushLogged("event counts lost on shutdown")
}

func (r *Recorder) flushLogged(message string) {
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()

	if err := r.Flush(ctx); err != nil {
		r.logger.Warn(message, "err", err)
	}
}
