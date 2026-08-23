//go:build windows

package secrets

type unavailableFileStore struct{}

func newFileStore(string) Store { return unavailableFileStore{} }

func (unavailableFileStore) Get(string) (string, error) { return "", ErrNotFound }
func (unavailableFileStore) Set(string, string) error   { return ErrReadOnly }
func (unavailableFileStore) Delete(string) error        { return ErrReadOnly }
func (unavailableFileStore) Has(string) bool            { return false }

type backendChoice struct{}

func newBackendChoice(string) backendChoice { return backendChoice{} }
func (backendChoice) Read() (string, error) { return "", ErrNotFound }
func (backendChoice) Write(string) error    { return ErrReadOnly }
