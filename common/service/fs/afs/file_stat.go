package afs

type FileID struct {
	Device uint64
	Inode  uint64
}

type FileStat struct {
	ID    FileID
	Links uint64
}
