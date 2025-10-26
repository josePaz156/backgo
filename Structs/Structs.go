package Structs

//  =============================================================

type MBR struct {
	MbrSize      int32
	CreationDate [10]byte
	Signature    int32
	Fit          [2]byte
	Partitions   [4]Partition
}

//  =============================================================

type Partition struct {
	Status      [1]byte
	Type        [1]byte
	Fit         [2]byte
	Start       int32
	Size        int32
	Name        [16]byte
	Correlative int32
	Id          [4]byte
}

//  =============================================================

type EBR struct {
    Part_status    [1]byte
    Part_fit       [1]byte  
    Part_start     int32
    Part_size      int32
    Part_next      int32    
    Part_name      [16]byte
}

//  =============================================================

type Superblock struct {
	S_filesystem_type   int32
	S_inodes_count      int32  
	S_blocks_count      int32
	S_free_blocks_count int32
	S_free_inodes_count int32
	S_mtime             [17]byte
	S_umtime            [17]byte
	S_mnt_count         int32
	S_magic             int32
	S_inode_size        int32
	S_block_size        int32
	S_fist_ino          int32
	S_first_blo         int32
	S_bm_inode_start    int32
	S_bm_block_start    int32
	S_inode_start       int32
	S_block_start       int32
}

//  =============================================================

type Inode struct {
	I_uid   int32
	I_gid   int32
	I_size  int32
	I_atime [17]byte
	I_ctime [17]byte
	I_mtime [17]byte
	I_block [15]int32
	I_type  [1]byte
	I_perm  [3]byte
}

//  =============================================================

type Fileblock struct {
	B_content [64]byte
}

//  =============================================================

type Content struct {
	B_name  [12]byte
	B_inodo int32
}

type Folderblock struct {
	B_content [4]Content
}

//  =============================================================

type Pointerblock struct {
	B_pointers [16]int32
}

//  =============================================================


type Information struct {
	Operation [10]byte  
	Path      [32]byte  
	Content   [64]byte  
	Date      float32   
}

type Journaling struct {
	Count   int32       
	Content Information 
}

// Estructura para representar una partición montada
type MountedPartition struct {
	Path           string 
	PartitionName  string 
	Id             string 
	PartitionIndex int    
	IsLogical      bool   
	EBRPosition    int32  
}

// Contadores para generar IDs únicos por disco
type DiskCounters struct {
	PartitionNumber int  
	Letter          byte 
}

// Estructura para representar una sesión de usuario activa
type UserSession struct {
	Username     string 
	UserID       int    
	GroupID      int
	PartitionID  string 
	IsActive     bool   
}

// Estructura para representar un usuario del sistema
type SystemUser struct {
	ID       int
	Type     string 
	Group    string
	Username string
	Password string
}

// Estructura para representar un grupo del sistema
type SystemGroup struct {
	ID   int
	Type string 
	Name string
}

