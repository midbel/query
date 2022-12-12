package comma

type object struct {
	fields map[string]Indexer
	keys   []string
}

func (o *object) Index(row []string) error {
	return nil
}

type array struct {
	list []Indexer
}

func (a *array) Index(row []string) error {
	return nil
}

type literal struct {
	value string
}

func (i *literal) Index([]string) error {
	return nil
}

type index struct {
	index int
}

func (i *index) Index(row []string) error {
	return nil
}

type set struct {
	index []Indexer
}

func (i *set) Index(row []string) error {
	return nil
}

type interval struct {
	beg int
	end int
}

func (i *interval) Index(row []string) error {
	return nil
}
