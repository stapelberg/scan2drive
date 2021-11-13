package scaningest

type Page []byte

type Ingester struct {
	IngestCallback func(*Job) error
}

type Job struct {
	ingester *Ingester
	Pages    []Page
}

func (i *Ingester) NewJob() (*Job, error) {
	return &Job{ingester: i}, nil
}

func (j *Job) AddPage(page Page) error {
	// NOTE: For large jobs, we’d need to spill pages to disk to not exhaust
	// memory. For now, we just assume small-enough jobs, and/or enough RAM.
	j.Pages = append(j.Pages, page)
	return nil
}

func (j *Job) Ingest() error {
	return j.ingester.IngestCallback(j)
}
