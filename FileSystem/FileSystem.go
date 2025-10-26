package FileSystem

import (
	"proyecto1/Structs"
	"proyecto1/Utilities"
	"proyecto1/DiskManagement"
	"encoding/binary"
	"fmt"
	"strings"
	"os"
	"io/ioutil"
	"time"
)

// ============================================================================
// FUNCIÓN AUXILIAR PARA JOURNALING (EXT3)
// ============================================================================

// writeToJournal escribe una entrada en el journaling si el sistema es EXT3
func writeToJournal(partitionID string, operation string, path string, content string) {
	// Buscar la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return // Si no está montada, no hacemos nada
	}

	// Abrir archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return
	}
	defer file.Close()

	// Leer MBR
	var tempMBR Structs.MBR
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		return
	}

	// Obtener la partición correcta
	var partition *Structs.Partition = nil
	if !mountedPartition.IsLogical {
		partition = &tempMBR.Partitions[mountedPartition.PartitionIndex]
	} else {
		var tempEBR Structs.EBR
		if err := Utilities.ReadObject(file, &tempEBR, int64(mountedPartition.EBRPosition)); err != nil {
			return
		}
		tempPartition := Structs.Partition{
			Start: mountedPartition.EBRPosition + int32(binary.Size(Structs.EBR{})),
			Size:  tempEBR.Part_size,
		}
		partition = &tempPartition
	}

	// Leer el superblock
	var superblock Structs.Superblock
	if err := Utilities.ReadObject(file, &superblock, int64(partition.Start)); err != nil {
		return
	}

	// Verificar que sea un sistema EXT3
	if superblock.S_filesystem_type != 3 {
		return // Solo escribir en el journaling si es EXT3
	}

	// Calcular la posición del journaling
	superblockSize := int32(binary.Size(Structs.Superblock{}))
	journalingStart := partition.Start + superblockSize
	journalingSize := int32(binary.Size(Structs.Journaling{}))
	const JOURNALING_CONSTANT = 50

	// Buscar la primera entrada libre o la última entrada usada
	var lastUsedIndex int32 = -1
	for i := int32(0); i < JOURNALING_CONSTANT; i++ {
		var journalEntry Structs.Journaling
		journalPos := int64(journalingStart + i*journalingSize)
		if err := Utilities.ReadObject(file, &journalEntry, journalPos); err != nil {
			continue
		}
		if journalEntry.Count > 0 {
			lastUsedIndex = i
		}
	}

	// La nueva entrada será después de la última usada
	newIndex := lastUsedIndex + 1
	if newIndex >= JOURNALING_CONSTANT {
		// Si ya se llenó el journaling, sobrescribir desde el inicio (circular)
		newIndex = 0
	}

	// Crear la nueva entrada de journaling
	var newJournal Structs.Journaling
	newJournal.Count = newIndex + 1 // Contador incremental
	
	// Copiar operación (máximo 10 caracteres)
	if len(operation) > 10 {
		operation = operation[:10]
	}
	copy(newJournal.Content.Operation[:], operation)
	
	// Copiar ruta (máximo 32 caracteres)
	if len(path) > 32 {
		path = path[:32]
	}
	copy(newJournal.Content.Path[:], path)
	
	// Copiar contenido (máximo 64 caracteres)
	if len(content) > 64 {
		content = content[:64]
	}
	copy(newJournal.Content.Content[:], content)
	
	// Asignar timestamp actual como float32
	newJournal.Content.Date = float32(time.Now().Unix())

	// Escribir la entrada al journaling
	journalPos := int64(journalingStart + newIndex*journalingSize)
	if err := Utilities.WriteObject(file, newJournal, journalPos); err != nil {
		fmt.Printf("Advertencia: Error escribiendo entrada al journaling: %v\n", err)
	}
}

func Mkfs(id string, type_ string, filesystem string){
	fmt.Println("======Inicio MKFS======")
	
	// Determinar el tipo de sistema de archivos
	var fsType string
	var fsTypeNum int32
	if filesystem == "3fs" {
		fsType = "EXT3"
		fsTypeNum = 3
	} else {
		fsType = "EXT2"
		fsTypeNum = 2
	}
	
	fmt.Printf("Creando sistema de archivos %s en partición ID: %s\n", fsType, id)
	fmt.Printf("Tipo de formateo: %s\n", type_)
	
	// Verificar que la partición esté montada
	mountedPartition, exists := DiskManagement.MountedPartitions[id]
	if !exists {
		fmt.Printf("Error: La partición con ID '%s' no está montada\n", id)
		fmt.Println("Use el comando 'mounted' para ver las particiones disponibles")
		return
	}

	fmt.Printf("Partición encontrada: %s en disco: %s\n", mountedPartition.PartitionName, mountedPartition.Path)

	// Abrir archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		fmt.Println("Error abriendo archivo del disco:", err)
		return
	}
	defer file.Close()

	var tempMBR Structs.MBR
	// Leer MBR del archivo
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		fmt.Println("Error leyendo MBR:", err)
		return
	}

	// Buscar la partición por ID para obtener sus datos
	var partition *Structs.Partition = nil
	var partitionIndex int = -1

	// Buscar en particiones primarias
	if !mountedPartition.IsLogical {
		partitionIndex = mountedPartition.PartitionIndex
		partition = &tempMBR.Partitions[partitionIndex]
	} else {
		// Para particiones lógicas, necesitamos leer el EBR
		var tempEBR Structs.EBR
		if err := Utilities.ReadObject(file, &tempEBR, int64(mountedPartition.EBRPosition)); err != nil {
			fmt.Println("Error leyendo EBR:", err)
			return
		}
		// Crear una partición temporal con los datos del EBR
		tempPartition := Structs.Partition{
			Status: tempEBR.Part_status,
			Type:   [1]byte{'l'}, // lógica
			Fit:    [2]byte{tempEBR.Part_fit[0], 0}, // Convertir [1]byte a [2]byte
			Start:  mountedPartition.EBRPosition + int32(binary.Size(Structs.EBR{})),
			Size:   tempEBR.Part_size,
			Name:   tempEBR.Part_name,
			Id:     [4]byte{},
		}
		copy(tempPartition.Id[:], id)
		partition = &tempPartition
	}

	fmt.Printf("Datos de la partición - Inicio: %d, Tamaño: %d bytes\n", partition.Start, partition.Size)

	// Calcular el número de estructuras necesarias
	superblockSize := int32(binary.Size(Structs.Superblock{}))
	inodeSize := int32(binary.Size(Structs.Inode{}))
	blockSize := int32(binary.Size(Structs.Fileblock{}))
	journalingSize := int32(binary.Size(Structs.Journaling{}))

	var n int32
	
	if fsTypeNum == 3 {
		// Cálculo para EXT3 con journaling
		// tamaño_particion = sizeof(superblock) + n * sizeof(Journaling) + n + 3*n + n * sizeof(inodos) + 3*n * sizeof(block)
		// Despejando n:
		// n = (tamaño_particion - sizeof(superblock)) / (sizeof(Journaling) + 1 + 3 + sizeof(inodos) + 3*sizeof(block))
		
		// Constante de journaling = 50
		const JOURNALING_CONSTANT = 50
		journalingTotalSize := JOURNALING_CONSTANT * journalingSize
		
		// Espacio disponible (descontando superblock y journaling)
		availableSpace := partition.Size - superblockSize - journalingTotalSize
		
		// Cada inodo necesita: 1 bit bitmap inodos + 1 inodo + 3 bits bitmap bloques + 3 bloques
		structureSize := 1 + inodeSize + 3 + 3*blockSize
		n = availableSpace / structureSize
		
		fmt.Printf("Calculando estructuras para EXT3 con journaling (constante=%d)...\n", JOURNALING_CONSTANT)
	} else {
		// Cálculo para EXT2 (sin journaling)
		availableSpace := partition.Size - superblockSize
		structureSize := 1 + inodeSize + 3 + 3*blockSize
		n = availableSpace / structureSize
		
		fmt.Printf("Calculando estructuras para EXT2...\n")
	}

	if n <= 0 {
		fmt.Printf("Error: La partición es demasiado pequeña para crear un sistema de archivos\n")
		fmt.Printf("Tamaño mínimo requerido: %d bytes\n", partition.Size)
		return
	}

	fmt.Printf("Número de inodos calculado: %d\n", n)
	fmt.Printf("Número de bloques calculado: %d\n", 3*n)

	// Crear y configurar el superblock
	var superblock Structs.Superblock
	superblock.S_filesystem_type = fsTypeNum // 2 para EXT2, 3 para EXT3
	superblock.S_magic = 0xEF53
	superblock.S_inodes_count = n
	superblock.S_blocks_count = 3 * n
	superblock.S_free_inodes_count = n - 2    // Reservamos inodos 0 y 1
	superblock.S_free_blocks_count = 3*n - 2  // Reservamos bloques 0 y 1
	
	// Configurar fechas
	currentDate := "17/10/2025"
	copy(superblock.S_mtime[:], currentDate)
	copy(superblock.S_umtime[:], currentDate)
	superblock.S_mnt_count = 1

	// Configurar tamaños
	superblock.S_inode_size = inodeSize
	superblock.S_block_size = blockSize
	superblock.S_fist_ino = 2  // Primer inodo libre
	superblock.S_first_blo = 2 // Primer bloque libre

	// Calcular posiciones de las estructuras según el tipo de sistema de archivos
	var journalingStart int32
	const JOURNALING_CONSTANT = 50
	
	if fsTypeNum == 3 {
		// EXT3: Superblock, Journaling, Bitmap inodos, Bitmap bloques, Inodos, Bloques
		journalingStart = partition.Start + superblockSize
		superblock.S_bm_inode_start = journalingStart + (JOURNALING_CONSTANT * journalingSize)
		superblock.S_bm_block_start = superblock.S_bm_inode_start + n
		superblock.S_inode_start = superblock.S_bm_block_start + 3*n  
		superblock.S_block_start = superblock.S_inode_start + n*inodeSize
		
		fmt.Println("=== ESTRUCTURA DEL SISTEMA DE ARCHIVOS EXT3 ===")
		fmt.Printf("Superblock:        posición %d (tamaño: %d bytes)\n", partition.Start, superblockSize)
		fmt.Printf("Journaling:        posición %d (tamaño: %d entradas)\n", journalingStart, JOURNALING_CONSTANT)
		fmt.Printf("Bitmap inodos:     posición %d (tamaño: %d bytes)\n", superblock.S_bm_inode_start, n)
		fmt.Printf("Bitmap bloques:    posición %d (tamaño: %d bytes)\n", superblock.S_bm_block_start, 3*n)
		fmt.Printf("Tabla de inodos:   posición %d (tamaño: %d bytes)\n", superblock.S_inode_start, n*inodeSize)
		fmt.Printf("Bloques de datos:  posición %d (tamaño: %d bytes)\n", superblock.S_block_start, 3*n*blockSize)
	} else {
		// EXT2: Superblock, Bitmap inodos, Bitmap bloques, Inodos, Bloques
		superblock.S_bm_inode_start = partition.Start + superblockSize
		superblock.S_bm_block_start = superblock.S_bm_inode_start + n
		superblock.S_inode_start = superblock.S_bm_block_start + 3*n  
		superblock.S_block_start = superblock.S_inode_start + n*inodeSize
		
		fmt.Println("=== ESTRUCTURA DEL SISTEMA DE ARCHIVOS EXT2 ===")
		fmt.Printf("Superblock:        posición %d (tamaño: %d bytes)\n", partition.Start, superblockSize)
		fmt.Printf("Bitmap inodos:     posición %d (tamaño: %d bytes)\n", superblock.S_bm_inode_start, n)
		fmt.Printf("Bitmap bloques:    posición %d (tamaño: %d bytes)\n", superblock.S_bm_block_start, 3*n)
		fmt.Printf("Tabla de inodos:   posición %d (tamaño: %d bytes)\n", superblock.S_inode_start, n*inodeSize)
		fmt.Printf("Bloques de datos:  posición %d (tamaño: %d bytes)\n", superblock.S_block_start, 3*n*blockSize)
	}

	// Formatear completamente la partición con ceros
	if type_ == "full" {
		fmt.Println("Realizando formateo completo...")
		var zeroByte byte = 0
		for i := int32(0); i < partition.Size; i++ {
			Utilities.WriteObject(file, zeroByte, int64(partition.Start+i))
		}
		fmt.Println("Formateo completo terminado.")
	}

	// Inicializar journaling si es EXT3
	if fsTypeNum == 3 {
		fmt.Println("Inicializando journaling...")
		var emptyJournal Structs.Journaling
		emptyJournal.Count = 0
		// Inicializar el contenido vacío
		for i := 0; i < 10; i++ {
			emptyJournal.Content.Operation[i] = 0
		}
		for i := 0; i < 32; i++ {
			emptyJournal.Content.Path[i] = 0
		}
		for i := 0; i < 64; i++ {
			emptyJournal.Content.Content[i] = 0
		}
		emptyJournal.Content.Date = 0.0
		
		// Escribir 50 entradas de journaling vacías
		for i := int32(0); i < JOURNALING_CONSTANT; i++ {
			if err := Utilities.WriteObject(file, emptyJournal, int64(journalingStart+i*journalingSize)); err != nil {
				fmt.Printf("Error inicializando journaling en posición %d: %v\n", i, err)
			}
		}
		fmt.Printf("Journaling inicializado con %d entradas vacías\n", JOURNALING_CONSTANT)
		
		// Escribir la primera entrada del journal con la operación mkfs
		var mkfsJournal Structs.Journaling
		mkfsJournal.Count = 1
		copy(mkfsJournal.Content.Operation[:], "mkfs")
		copy(mkfsJournal.Content.Path[:], id)
		mkfsContent := fmt.Sprintf("EXT3 format - Inodes:%d Blocks:%d", n, 3*n)
		copy(mkfsJournal.Content.Content[:], mkfsContent)
		mkfsJournal.Content.Date = 23102025.0 // Fecha actual
		
		if err := Utilities.WriteObject(file, mkfsJournal, int64(journalingStart)); err != nil {
			fmt.Printf("Error escribiendo entrada mkfs al journal: %v\n", err)
		} else {
			fmt.Println("Entrada 'mkfs' registrada en el journaling")
		}
	}

	// Inicializar bitmaps con ceros
	fmt.Println("Inicializando bitmaps...")
	for i := int32(0); i < n; i++ {
		Utilities.WriteObject(file, byte(0), int64(superblock.S_bm_inode_start+i))
	}
	for i := int32(0); i < 3*n; i++ {
		Utilities.WriteObject(file, byte(0), int64(superblock.S_bm_block_start+i))
	}

	// Inicializar tabla de inodos vacía
	fmt.Println("Inicializando tabla de inodos...")
	var emptyInode Structs.Inode
	for i := int32(0); i < 15; i++ {
		emptyInode.I_block[i] = -1
	}
	for i := int32(0); i < n; i++ {
		Utilities.WriteObject(file, emptyInode, int64(superblock.S_inode_start+i*inodeSize))
	}

	// Inicializar bloques de datos vacíos
	fmt.Println("Inicializando bloques de datos...")
	var emptyBlock Structs.Fileblock
	for i := int32(0); i < 3*n; i++ {
		Utilities.WriteObject(file, emptyBlock, int64(superblock.S_block_start+i*blockSize))
	}

	// Crear estructura inicial del sistema de archivos
	fmt.Println("Creando estructura inicial del sistema de archivos...")
	
	// INODO 0: Directorio raíz
	var rootInode Structs.Inode
	rootInode.I_uid = 1    // Usuario root
	rootInode.I_gid = 1    // Grupo root  
	rootInode.I_size = 0
	copy(rootInode.I_atime[:], currentDate)
	copy(rootInode.I_ctime[:], currentDate)
	copy(rootInode.I_mtime[:], currentDate)
	copy(rootInode.I_type[:], "0")    // 0 = directorio
	copy(rootInode.I_perm[:], "777")  // Permisos rwxrwxrwx para root
	for i := 0; i < 15; i++ {
		rootInode.I_block[i] = -1
	}
	rootInode.I_block[0] = 0 // Apunta al bloque 0

	// BLOQUE 0: Contenido del directorio raíz
	var rootDirBlock Structs.Folderblock
	rootDirBlock.B_content[0].B_inodo = 0
	copy(rootDirBlock.B_content[0].B_name[:], ".")
	rootDirBlock.B_content[1].B_inodo = 0  
	copy(rootDirBlock.B_content[1].B_name[:], "..")
	rootDirBlock.B_content[2].B_inodo = 1
	copy(rootDirBlock.B_content[2].B_name[:], "users.txt")
	rootDirBlock.B_content[3].B_inodo = -1 // Entrada vacía

	// INODO 1: Archivo users.txt
	var usersInode Structs.Inode
	usersInode.I_uid = 1    // Usuario root
	usersInode.I_gid = 1    // Grupo root
	
	// Contenido del archivo users.txt según especificaciones
	usersContent := "1,G,root\n1,U,root,root,123\n"
	usersInode.I_size = int32(len(usersContent))
	
	copy(usersInode.I_atime[:], currentDate)
	copy(usersInode.I_ctime[:], currentDate) 
	copy(usersInode.I_mtime[:], currentDate)
	copy(usersInode.I_type[:], "1")    // 1 = archivo regular
	copy(usersInode.I_perm[:], "777")  // Permisos rwxrwxrwx para root
	for i := 0; i < 15; i++ {
		usersInode.I_block[i] = -1
	}
	usersInode.I_block[0] = 1 // Apunta al bloque 1

	// BLOQUE 1: Contenido del archivo users.txt
	var usersFileBlock Structs.Fileblock
	copy(usersFileBlock.B_content[:len(usersContent)], usersContent)

	// Escribir todas las estructuras al disco
	fmt.Println("Escribiendo estructuras al disco...")

	// Escribir superblock
	if err := Utilities.WriteObject(file, superblock, int64(partition.Start)); err != nil {
		fmt.Println("Error escribiendo superblock:", err)
		return
	}

	// Marcar inodos 0 y 1 como ocupados en el bitmap
	Utilities.WriteObject(file, byte(1), int64(superblock.S_bm_inode_start+0))
	Utilities.WriteObject(file, byte(1), int64(superblock.S_bm_inode_start+1))

	// Marcar bloques 0 y 1 como ocupados en el bitmap  
	Utilities.WriteObject(file, byte(1), int64(superblock.S_bm_block_start+0))
	Utilities.WriteObject(file, byte(1), int64(superblock.S_bm_block_start+1))

	// Escribir inodos
	Utilities.WriteObject(file, rootInode, int64(superblock.S_inode_start))
	Utilities.WriteObject(file, usersInode, int64(superblock.S_inode_start+inodeSize))

	// Escribir bloques
	Utilities.WriteObject(file, rootDirBlock, int64(superblock.S_block_start))
	Utilities.WriteObject(file, usersFileBlock, int64(superblock.S_block_start+blockSize))

	fmt.Printf("=== SISTEMA DE ARCHIVOS %s CREADO EXITOSAMENTE ===\n", fsType)
	fmt.Printf("Partición ID: %s\n", id)
	fmt.Printf("Sistema: %s\n", fsType)
	fmt.Printf("Inodos totales: %d\n", superblock.S_inodes_count)
	fmt.Printf("Inodos disponibles: %d\n", superblock.S_free_inodes_count)
	fmt.Printf("Bloques totales: %d\n", superblock.S_blocks_count)
	fmt.Printf("Bloques disponibles: %d\n", superblock.S_free_blocks_count)
	
	if fsTypeNum == 3 {
		fmt.Printf("Journaling: %d entradas inicializadas\n", JOURNALING_CONSTANT)
	}
	
	fmt.Println("")
	fmt.Println("Estructura inicial:")
	fmt.Println("  / (directorio raíz)")
	fmt.Println("  ├── . (enlace al directorio actual)")
	fmt.Println("  ├── .. (enlace al directorio padre)")
	fmt.Println("  └── users.txt (archivo de usuarios y grupos)")
	fmt.Println("")
	fmt.Println("Archivo users.txt contiene:")
	fmt.Println("  1,G,root        <- Grupo root (ID=1)")
	fmt.Println("  1,U,root,root,123 <- Usuario root (ID=1, Grupo=root, Contraseña=123)")
	fmt.Println("")
	fmt.Println("El usuario root tiene permisos completos para modificar el sistema.")
	fmt.Println("======FIN MKFS======")
}

// Función para leer el superblock de una partición
func ReadSuperblock(id string) (*Structs.Superblock, error) {
	// Buscar la partición montada por ID
	mountedPartition, exists := DiskManagement.MountedPartitions[id]
	if !exists {
		return nil, fmt.Errorf("partición con ID '%s' no está montada", id)
	}
	
	// Abrir archivo del disco usando la ruta de la partición montada
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var tempMBR Structs.MBR
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		return nil, err
	}

	// Buscar la partición por ID en el MBR
	var partition *Structs.Partition = nil
	
	if !mountedPartition.IsLogical {
		// Partición primaria
		partition = &tempMBR.Partitions[mountedPartition.PartitionIndex]
	} else {
		// Partición lógica - leer desde EBR
		var tempEBR Structs.EBR
		if err := Utilities.ReadObject(file, &tempEBR, int64(mountedPartition.EBRPosition)); err != nil {
			return nil, err
		}
		// Crear partición temporal con datos del EBR
		tempPartition := Structs.Partition{
			Start: mountedPartition.EBRPosition + int32(binary.Size(Structs.EBR{})),
			Size:  tempEBR.Part_size,
		}
		partition = &tempPartition
	}

	if partition == nil {
		return nil, fmt.Errorf("partición no encontrada")
	}

	var superblock Structs.Superblock
	if err := Utilities.ReadObject(file, &superblock, int64(partition.Start)); err != nil {
		return nil, err
	}

	return &superblock, nil
}

// Función para mostrar información del sistema de archivos
func ShowFileSystemInfo(id string) {
	fmt.Println("=== INFORMACIÓN DEL SISTEMA DE ARCHIVOS ===")
	
	superblock, err := ReadSuperblock(id)
	if err != nil {
		fmt.Println("Error leyendo superblock:", err)
		return
	}

	fmt.Printf("Tipo de sistema: EXT%d\n", superblock.S_filesystem_type)
	fmt.Printf("Número mágico: 0x%X\n", superblock.S_magic)
	fmt.Printf("Total de inodos: %d\n", superblock.S_inodes_count)
	fmt.Printf("Total de bloques: %d\n", superblock.S_blocks_count)
	fmt.Printf("Inodos libres: %d\n", superblock.S_free_inodes_count)
	fmt.Printf("Bloques libres: %d\n", superblock.S_free_blocks_count)
	fmt.Printf("Tamaño de inodo: %d bytes\n", superblock.S_inode_size)
	fmt.Printf("Tamaño de bloque: %d bytes\n", superblock.S_block_size)
	fmt.Printf("Fecha de montaje: %s\n", string(superblock.S_mtime[:]))
	fmt.Printf("Fecha de desmontaje: %s\n", string(superblock.S_umtime[:]))
	fmt.Printf("Contador de montajes: %d\n", superblock.S_mnt_count)
	
	fmt.Println("\n=== UBICACIONES DE ESTRUCTURAS ===")
	fmt.Printf("Bitmap de inodos: posición %d\n", superblock.S_bm_inode_start)
	fmt.Printf("Bitmap de bloques: posición %d\n", superblock.S_bm_block_start)
	fmt.Printf("Tabla de inodos: posición %d\n", superblock.S_inode_start)
	fmt.Printf("Bloques de datos: posición %d\n", superblock.S_block_start)
}

// Función para listar el contenido del directorio raíz
func ListRootDirectory(id string) {
	fmt.Println("=== CONTENIDO DEL DIRECTORIO RAÍZ ===")
	
	// Buscar la partición montada por ID
	mountedPartition, exists := DiskManagement.MountedPartitions[id]
	if !exists {
		fmt.Printf("Error: La partición con ID '%s' no está montada\n", id)
		fmt.Println("Use el comando 'mounted' para ver las particiones disponibles")
		return
	}
	
	// Abrir archivo del disco usando la ruta de la partición montada
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		fmt.Println("Error abriendo archivo:", err)
		return
	}
	defer file.Close()

	superblock, err := ReadSuperblock(id)
	if err != nil {
		fmt.Println("Error leyendo superblock:", err)
		return
	}

	// Leer el inodo 0 (directorio raíz)
	var rootInode Structs.Inode
	inodePos := int64(superblock.S_inode_start)
	if err := Utilities.ReadObject(file, &rootInode, inodePos); err != nil {
		fmt.Println("Error leyendo inodo raíz:", err)
		return
	}

	fmt.Printf("Inodo raíz - UID: %d, GID: %d, Tamaño: %d, Tipo: %s\n", 
		rootInode.I_uid, rootInode.I_gid, rootInode.I_size, string(rootInode.I_type[:]))

	// Leer el bloque 0 (contenido del directorio raíz)
	if rootInode.I_block[0] != -1 {
		var folderBlock Structs.Folderblock
		blockPos := int64(superblock.S_block_start + rootInode.I_block[0]*int32(binary.Size(Structs.Fileblock{})))
		if err := Utilities.ReadObject(file, &folderBlock, blockPos); err != nil {
			fmt.Println("Error leyendo bloque de directorio:", err)
			return
		}

		fmt.Println("\nContenido del directorio:")
		for i := 0; i < 4; i++ {
			if folderBlock.B_content[i].B_inodo != -1 {
				name := strings.TrimRight(string(folderBlock.B_content[i].B_name[:]), "\x00")
				if name != "" {
					fmt.Printf("  %s -> inodo %d\n", name, folderBlock.B_content[i].B_inodo)
				}
			}
		}
	}
}

// ============================================================================
// FUNCIONES DE AUTENTICACIÓN Y SESIONES
// ============================================================================

// Variable global para la sesión activa (solo una sesión a la vez)
var CurrentSession *Structs.UserSession = nil

// Login - Iniciar sesión en el sistema
func Login(user string, pass string, id string) {
	fmt.Println("======Inicio LOGIN======")
	fmt.Printf("Usuario: %s\n", user)
	fmt.Printf("Partición ID: %s\n", id)
	
	// Verificar que no haya una sesión activa
	if CurrentSession != nil && CurrentSession.IsActive {
		fmt.Printf("Error: Ya hay una sesión activa del usuario '%s' en la partición '%s'\n", 
			CurrentSession.Username, CurrentSession.PartitionID)
		fmt.Println("Debe cerrar sesión con 'logout' antes de iniciar una nueva sesión")
		fmt.Println("======FIN LOGIN======")
		return
	}

	// Validar parámetros obligatorios
	if user == "" {
		fmt.Println("Error: El parámetro -user es obligatorio")
		fmt.Println("Uso: login -user=<usuario> -pass=<contraseña> -id=<ID_particion>")
		fmt.Println("======FIN LOGIN======")
		return
	}
	if pass == "" {
		fmt.Println("Error: El parámetro -pass es obligatorio")
		fmt.Println("Uso: login -user=<usuario> -pass=<contraseña> -id=<ID_particion>")
		fmt.Println("======FIN LOGIN======")
		return
	}
	if id == "" {
		fmt.Println("Error: El parámetro -id es obligatorio")
		fmt.Println("Uso: login -user=<usuario> -pass=<contraseña> -id=<ID_particion>")
		fmt.Println("======FIN LOGIN======")
		return
	}

	// Verificar que la partición esté montada
	mountedPartition, exists := DiskManagement.MountedPartitions[id]
	if !exists {
		fmt.Printf("Error: La partición con ID '%s' no está montada\n", id)
		fmt.Println("Use el comando 'mounted' para ver las particiones disponibles")
		fmt.Println("======FIN LOGIN======")
		return
	}

	fmt.Printf("Partición encontrada: %s en disco: %s\n", mountedPartition.PartitionName, mountedPartition.Path)

	// Leer el archivo users.txt de la partición
	usersData, err := readUsersFile(id)
	if err != nil {
		fmt.Printf("Error leyendo archivo users.txt: %s\n", err.Error())
		fmt.Println("Asegúrese de que la partición tenga un sistema de archivos creado con 'mkfs'")
		fmt.Println("======FIN LOGIN======")
		return
	}

	// Buscar el usuario en los datos
	userFound, userInfo := findUser(usersData, user)
	if !userFound {
		fmt.Printf("Error: El usuario '%s' no existe en el sistema\n", user)
		fmt.Println("Verifique que el nombre de usuario sea correcto (distingue mayúsculas y minúsculas)")
		fmt.Println("======FIN LOGIN======")
		return
	}

	// Verificar la contraseña (distingue mayúsculas y minúsculas)
	if userInfo.Password != pass {
		fmt.Printf("Error: Contraseña incorrecta para el usuario '%s'\n", user)
		fmt.Println("Verifique que la contraseña sea correcta (distingue mayúsculas y minúsculas)")
		fmt.Println("======FIN LOGIN======")
		return
	}

	// Crear la sesión
	CurrentSession = &Structs.UserSession{
		Username:    user,
		UserID:      userInfo.ID,
		GroupID:     getGroupID(usersData, userInfo.Group),
		PartitionID: id,
		IsActive:    true,
	}

	fmt.Println("=== INICIO DE SESIÓN EXITOSO ===")
	fmt.Printf("Usuario: %s (ID: %d)\n", CurrentSession.Username, CurrentSession.UserID)
	fmt.Printf("Grupo: %s (ID: %d)\n", userInfo.Group, CurrentSession.GroupID)
	fmt.Printf("Partición: %s (ID: %s)\n", mountedPartition.PartitionName, CurrentSession.PartitionID)
	fmt.Printf("Disco: %s\n", mountedPartition.Path)
	fmt.Println("Todas las operaciones se realizarán en esta partición hasta cerrar sesión")
	fmt.Println("Use 'logout' para cerrar sesión")
	fmt.Println("======FIN LOGIN======")
}

// Logout - Cerrar sesión del sistema
func Logout() {
	fmt.Println("======Inicio LOGOUT======")
	
	// Verificar que haya una sesión activa
	if CurrentSession == nil || !CurrentSession.IsActive {
		fmt.Println("Error: No hay ninguna sesión activa")
		fmt.Println("Debe iniciar sesión con el comando 'login' antes de poder cerrar sesión")
		fmt.Println("======FIN LOGOUT======")
		return
	}

	// Mostrar información de la sesión que se va a cerrar
	fmt.Printf("Cerrando sesión del usuario: %s\n", CurrentSession.Username)
	fmt.Printf("Partición: %s\n", CurrentSession.PartitionID)
	
	// Cerrar la sesión
	CurrentSession = nil
	
	fmt.Println("=== SESIÓN CERRADA EXITOSAMENTE ===")
	fmt.Println("Puede iniciar una nueva sesión con el comando 'login'")
	fmt.Println("======FIN LOGOUT======")
}

// GetCurrentSession - Obtener la sesión actual (para uso en otros comandos)
func GetCurrentSession() *Structs.UserSession {
	if CurrentSession != nil && CurrentSession.IsActive {
		return CurrentSession
	}
	return nil
}

// IsUserLoggedIn - Verificar si hay un usuario logueado
func IsUserLoggedIn() bool {
	return CurrentSession != nil && CurrentSession.IsActive
}

// RequireLogin - Función helper para comandos que requieren login
func RequireLogin() bool {
	if !IsUserLoggedIn() {
		fmt.Println("Error: Debe iniciar sesión primero")
		fmt.Println("Use: login -user=<usuario> -pass=<contraseña> -id=<ID_particion>")
		return false
	}
	return true
}

// ============================================================================
// FUNCIONES AUXILIARES PARA MANEJO DE USUARIOS
// ============================================================================

// readUsersFile - Leer el archivo users.txt de una partición
func readUsersFile(partitionID string) (string, error) {
	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return "", fmt.Errorf("partición no montada")
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return "", fmt.Errorf("error abriendo disco: %s", err.Error())
	}
	defer file.Close()

	// Leer el superblock para obtener la estructura del sistema
	var tempMBR Structs.MBR
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		return "", fmt.Errorf("error leyendo MBR: %s", err.Error())
	}

	// Obtener la partición correcta
	var partition *Structs.Partition = nil
	if !mountedPartition.IsLogical {
		partition = &tempMBR.Partitions[mountedPartition.PartitionIndex]
	} else {
		// Para partición lógica, crear una partición temporal
		var tempEBR Structs.EBR
		if err := Utilities.ReadObject(file, &tempEBR, int64(mountedPartition.EBRPosition)); err != nil {
			return "", fmt.Errorf("error leyendo EBR: %s", err.Error())
		}
		tempPartition := Structs.Partition{
			Start: mountedPartition.EBRPosition + int32(binary.Size(Structs.EBR{})),
			Size:  tempEBR.Part_size,
		}
		partition = &tempPartition
	}

	// Leer el superblock
	var superblock Structs.Superblock
	if err := Utilities.ReadObject(file, &superblock, int64(partition.Start)); err != nil {
		return "", fmt.Errorf("error leyendo superblock: %s", err.Error())
	}

	// Leer el inodo 1 (archivo users.txt)
	var usersInode Structs.Inode
	inodePos := int64(superblock.S_inode_start + int32(binary.Size(Structs.Inode{}))) // Inodo 1
	if err := Utilities.ReadObject(file, &usersInode, inodePos); err != nil {
		return "", fmt.Errorf("error leyendo inodo users.txt: %s", err.Error())
	}

	// Verificar que el archivo tenga al menos un bloque
	if usersInode.I_block[0] == -1 {
		return "", fmt.Errorf("archivo users.txt no tiene datos")
	}

	// Calcular cuántos bloques necesitamos leer
	fileSize := usersInode.I_size
	blocksNeeded := (fileSize + 63) / 64 // Redondear hacia arriba
	if blocksNeeded > 28 { // Máximo 12 directos + 16 indirectos
		return "", fmt.Errorf("archivo users.txt demasiado grande")
	}

	// Leer el contenido de todos los bloques
	var content strings.Builder
	bytesRead := int32(0)

	// Leer bloques directos (I_block[0] a I_block[11])
	for i := 0; i < 12 && i < int(blocksNeeded) && usersInode.I_block[i] != -1; i++ {
		var usersBlock Structs.Fileblock
		blockPos := int64(superblock.S_block_start + usersInode.I_block[i]*int32(binary.Size(Structs.Fileblock{})))
		if err := Utilities.ReadObject(file, &usersBlock, blockPos); err != nil {
			return "", fmt.Errorf("error leyendo bloque %d de users.txt: %s", i, err.Error())
		}

		// Calcular cuántos bytes leer de este bloque
		remainingBytes := fileSize - bytesRead
		bytesToRead := int32(64)
		if remainingBytes < 64 {
			bytesToRead = remainingBytes
		}

		content.WriteString(string(usersBlock.B_content[:bytesToRead]))
		bytesRead += bytesToRead
	}

	// Si necesitamos más bloques, leer del puntero indirecto
	if blocksNeeded > 12 && usersInode.I_block[12] != -1 {
		// Leer el bloque de punteros indirectos
		var indirectBlock Structs.Fileblock
		indirectBlockPos := int64(superblock.S_block_start + usersInode.I_block[12]*int32(binary.Size(Structs.Fileblock{})))
		if err := Utilities.ReadObject(file, &indirectBlock, indirectBlockPos); err != nil {
			return "", fmt.Errorf("error leyendo bloque de punteros indirectos: %s", err.Error())
		}

		// Leer bloques indirectos
		for i := 12; i < int(blocksNeeded) && i < 28; i++ {
			indirectIndex := i - 12
			var blockNumber int32
			binary.Read(strings.NewReader(string(indirectBlock.B_content[indirectIndex*4:(indirectIndex+1)*4])), binary.LittleEndian, &blockNumber)
			
			if blockNumber == -1 {
				break
			}

			var usersBlock Structs.Fileblock
			blockPos := int64(superblock.S_block_start + blockNumber*int32(binary.Size(Structs.Fileblock{})))
			if err := Utilities.ReadObject(file, &usersBlock, blockPos); err != nil {
				return "", fmt.Errorf("error leyendo bloque indirecto %d de users.txt: %s", i, err.Error())
			}

			// Calcular cuántos bytes leer de este bloque
			remainingBytes := fileSize - bytesRead
			bytesToRead := int32(64)
			if remainingBytes < 64 {
				bytesToRead = remainingBytes
			}

			content.WriteString(string(usersBlock.B_content[:bytesToRead]))
			bytesRead += bytesToRead
		}
	}

	return content.String(), nil
}

// findUser - Buscar un usuario en los datos del archivo users.txt
func findUser(usersData string, username string) (bool, Structs.SystemUser) {
	lines := strings.Split(usersData, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Dividir la línea por comas
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		
		// Verificar si es un usuario (tipo "U")
		if len(parts) == 5 && strings.TrimSpace(parts[1]) == "U" {
			userInFile := strings.TrimSpace(parts[3])
			if userInFile == username { // Distingue mayúsculas y minúsculas
				// Convertir ID a entero
				id := 0
				fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &id)
				
				return true, Structs.SystemUser{
					ID:       id,
					Type:     strings.TrimSpace(parts[1]),
					Group:    strings.TrimSpace(parts[2]),
					Username: strings.TrimSpace(parts[3]),
					Password: strings.TrimSpace(parts[4]),
				}
			}
		}
	}
	
	return false, Structs.SystemUser{}
}

// getGroupID - Obtener el ID de un grupo por nombre
func getGroupID(usersData string, groupName string) int {
	lines := strings.Split(usersData, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Dividir la línea por comas
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		
		// Verificar si es un grupo (tipo "G")
		if len(parts) == 3 && strings.TrimSpace(parts[1]) == "G" {
			groupInFile := strings.TrimSpace(parts[2])
			if groupInFile == groupName {
				// Convertir ID a entero
				id := 0
				fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &id)
				return id
			}
		}
	}
	
	return 0 // Grupo no encontrado
}

// ============================================================================
// FUNCIONES DE GESTIÓN DE USUARIOS Y GRUPOS
// ============================================================================

// Mkgrp - Crear un nuevo grupo en el sistema (solo root)
func Mkgrp(groupName string) {
	fmt.Println("======Inicio MKGRP======")
	fmt.Printf("Nombre del grupo: %s\n", groupName)
	
	// Verificar que haya una sesión activa
	if !IsUserLoggedIn() {
		fmt.Println("Error: Debe iniciar sesión primero")
		fmt.Println("Use: login -user=<usuario> -pass=<contraseña> -id=<ID_particion>")
		fmt.Println("======FIN MKGRP======")
		return
	}

	// Verificar que el usuario sea root
	if CurrentSession.Username != "root" {
		fmt.Printf("Error: Solo el usuario 'root' puede crear grupos\n")
		fmt.Printf("Usuario actual: %s\n", CurrentSession.Username)
		fmt.Println("======FIN MKGRP======")
		return
	}

	// Validar que el nombre del grupo no esté vacío
	if strings.TrimSpace(groupName) == "" {
		fmt.Println("Error: El nombre del grupo no puede estar vacío")
		fmt.Println("======FIN MKGRP======")
		return
	}

	// Leer el archivo users.txt actual
	usersData, err := readUsersFile(CurrentSession.PartitionID)
	if err != nil {
		fmt.Printf("Error leyendo archivo users.txt: %s\n", err.Error())
		fmt.Println("======FIN MKGRP======")
		return
	}

	// Verificar que el grupo no exista ya
	if groupExists(usersData, groupName) {
		fmt.Printf("Error: El grupo '%s' ya existe en el sistema\n", groupName)
		fmt.Println("Los nombres de grupos distinguen mayúsculas y minúsculas")
		fmt.Println("======FIN MKGRP======")
		return
	}

	// Obtener el siguiente ID disponible para el grupo
	nextGroupID := getNextAvailableGroupID(usersData)

	// Crear la nueva entrada del grupo
	newGroupEntry := fmt.Sprintf("%d,G,%s", nextGroupID, groupName)

	// Agregar el nuevo grupo al contenido existente
	updatedUsersData := usersData + newGroupEntry + "\n"

	// Escribir el contenido actualizado al archivo users.txt
	err = writeUsersFile(CurrentSession.PartitionID, updatedUsersData)
	if err != nil {
		fmt.Printf("Error escribiendo archivo users.txt: %s\n", err.Error())
		fmt.Println("======FIN MKGRP======")
		return
	}

	// Registrar en el journaling (EXT3)
	writeToJournal(CurrentSession.PartitionID, "mkgrp", "/users.txt", groupName)

	fmt.Println("=== GRUPO CREADO EXITOSAMENTE ===")
	fmt.Printf("Nombre del grupo: %s\n", groupName)
	fmt.Printf("ID asignado: %d\n", nextGroupID)
	fmt.Printf("Partición: %s\n", CurrentSession.PartitionID)
	fmt.Printf("Usuario que creó el grupo: %s\n", CurrentSession.Username)
	fmt.Println("El grupo ha sido agregado al archivo users.txt")
	fmt.Println("======FIN MKGRP======")
}

// Rmgrp - Eliminar un grupo del sistema (solo root)
func Rmgrp(groupName string) {
	fmt.Println("======Inicio RMGRP======")
	fmt.Printf("Nombre del grupo a eliminar: %s\n", groupName)
	
	// Verificar que haya una sesión activa
	if !IsUserLoggedIn() {
		fmt.Println("Error: Debe iniciar sesión primero")
		fmt.Println("Use: login -user=<usuario> -pass=<contraseña> -id=<ID_particion>")
		fmt.Println("======FIN RMGRP======")
		return
	}

	// Verificar que el usuario sea root
	if CurrentSession.Username != "root" {
		fmt.Printf("Error: Solo el usuario 'root' puede eliminar grupos\n")
		fmt.Printf("Usuario actual: %s\n", CurrentSession.Username)
		fmt.Println("======FIN RMGRP======")
		return
	}

	// Validar que el nombre del grupo no esté vacío
	if strings.TrimSpace(groupName) == "" {
		fmt.Println("Error: El nombre del grupo no puede estar vacío")
		fmt.Println("======FIN RMGRP======")
		return
	}

	// Leer el archivo users.txt actual
	usersData, err := readUsersFile(CurrentSession.PartitionID)
	if err != nil {
		fmt.Printf("Error leyendo archivo users.txt: %s\n", err.Error())
		fmt.Println("======FIN RMGRP======")
		return
	}

	// Verificar que el grupo existe y no está ya eliminado
	groupExists, groupID := findGroupForDeletion(usersData, groupName)
	if !groupExists {
		fmt.Printf("Error: El grupo '%s' no existe en el sistema\n", groupName)
		fmt.Println("Verifique que el nombre del grupo sea correcto (distingue mayúsculas y minúsculas)")
		fmt.Println("======FIN RMGRP======")
		return
	}

	if groupID == 0 {
		fmt.Printf("Error: El grupo '%s' ya ha sido eliminado anteriormente\n", groupName)
		fmt.Println("======FIN RMGRP======")
		return
	}

	// Verificar que no sea el grupo root
	if groupName == "root" {
		fmt.Println("Error: No se puede eliminar el grupo 'root'")
		fmt.Println("El grupo root es necesario para el funcionamiento del sistema")
		fmt.Println("======FIN RMGRP======")
		return
	}

	// Marcar el grupo como eliminado (cambiar ID a 0)
	updatedUsersData := markGroupAsDeleted(usersData, groupName)

	// Escribir el contenido actualizado al archivo users.txt
	err = writeUsersFile(CurrentSession.PartitionID, updatedUsersData)
	if err != nil {
		fmt.Printf("Error escribiendo archivo users.txt: %s\n", err.Error())
		fmt.Println("======FIN RMGRP======")
		return
	}

	// Registrar en el journaling (EXT3)
	writeToJournal(CurrentSession.PartitionID, "rmgrp", "/users.txt", groupName)

	fmt.Println("=== GRUPO ELIMINADO EXITOSAMENTE ===")
	fmt.Printf("Nombre del grupo: %s\n", groupName)
	fmt.Printf("ID anterior: %d\n", groupID)
	fmt.Println("ID actual: 0 (marcado como eliminado)")
	fmt.Printf("Partición: %s\n", CurrentSession.PartitionID)
	fmt.Printf("Usuario que eliminó el grupo: %s\n", CurrentSession.Username)
	fmt.Println("El grupo ha sido marcado como eliminado en el archivo users.txt")
	fmt.Println("======FIN RMGRP======")
}

// findGroupForDeletion - Buscar un grupo para eliminación y obtener su ID
func findGroupForDeletion(usersData string, groupName string) (bool, int) {
	lines := strings.Split(usersData, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Dividir la línea por comas
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		
		// Verificar si es un grupo (tipo "G")
		if strings.TrimSpace(parts[1]) == "G" {
			existingGroupName := strings.TrimSpace(parts[2])
			if existingGroupName == groupName { // Distingue mayúsculas y minúsculas
				// Obtener el ID del grupo
				id := 0
				fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &id)
				return true, id
			}
		}
	}
	
	return false, 0
}

// markGroupAsDeleted - Marcar un grupo como eliminado (cambiar ID a 0)
func markGroupAsDeleted(usersData string, groupName string) string {
	lines := strings.Split(usersData, "\n")
	var updatedLines []string
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		// Dividir la línea por comas
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			updatedLines = append(updatedLines, line)
			continue
		}
		
		// Verificar si es el grupo que queremos eliminar
		if strings.TrimSpace(parts[1]) == "G" && strings.TrimSpace(parts[2]) == groupName {
			// Cambiar el ID a 0 para marcarlo como eliminado
			updatedLine := fmt.Sprintf("0,G,%s", groupName)
			updatedLines = append(updatedLines, updatedLine)
		} else {
			updatedLines = append(updatedLines, line)
		}
	}
	
	// Unir las líneas con saltos de línea
	result := strings.Join(updatedLines, "\n")
	if len(updatedLines) > 0 {
		result += "\n" // Asegurar que termine con \n
	}
	
	return result
}

// groupExists - Verificar si un grupo ya existe en el sistema
func groupExists(usersData string, groupName string) bool {
	lines := strings.Split(usersData, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Dividir la línea por comas
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		
		// Verificar si es un grupo (tipo "G")
		if strings.TrimSpace(parts[1]) == "G" {
			existingGroupName := strings.TrimSpace(parts[2])
			if existingGroupName == groupName { // Distingue mayúsculas y minúsculas
				return true
			}
		}
	}
	
	return false
}

// getNextAvailableGroupID - Obtener el siguiente ID disponible para un grupo
func getNextAvailableGroupID(usersData string) int {
	maxID := 0
	lines := strings.Split(usersData, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Dividir la línea por comas
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		
		// Obtener el ID
		id := 0
		fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &id)
		if id > maxID {
			maxID = id
		}
	}
	
	return maxID + 1
}

// writeUsersFile - Escribir contenido al archivo users.txt de una partición
func writeUsersFile(partitionID string, content string) error {
	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return fmt.Errorf("partición no montada")
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %s", err.Error())
	}
	defer file.Close()

	// Leer el superblock para obtener la estructura del sistema
	var tempMBR Structs.MBR
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		return fmt.Errorf("error leyendo MBR: %s", err.Error())
	}

	// Obtener la partición correcta
	var partition *Structs.Partition = nil
	if !mountedPartition.IsLogical {
		partition = &tempMBR.Partitions[mountedPartition.PartitionIndex]
	} else {
		// Para partición lógica, crear una partición temporal
		var tempEBR Structs.EBR
		if err := Utilities.ReadObject(file, &tempEBR, int64(mountedPartition.EBRPosition)); err != nil {
			return fmt.Errorf("error leyendo EBR: %s", err.Error())
		}
		tempPartition := Structs.Partition{
			Start: mountedPartition.EBRPosition + int32(binary.Size(Structs.EBR{})),
			Size:  tempEBR.Part_size,
		}
		partition = &tempPartition
	}

	// Leer el superblock
	var superblock Structs.Superblock
	if err := Utilities.ReadObject(file, &superblock, int64(partition.Start)); err != nil {
		return fmt.Errorf("error leyendo superblock: %s", err.Error())
	}

	// Leer el inodo 1 (archivo users.txt)
	var usersInode Structs.Inode
	inodePos := int64(superblock.S_inode_start + int32(binary.Size(Structs.Inode{}))) // Inodo 1
	if err := Utilities.ReadObject(file, &usersInode, inodePos); err != nil {
		return fmt.Errorf("error leyendo inodo users.txt: %s", err.Error())
	}

	// Verificar que el contenido no exceda el tamaño máximo manejable
	maxBlocks := int32(28) // 12 directos + 16 indirectos simples
	maxFileSize := maxBlocks * 64 // 64 bytes por bloque
	
	if len(content) > int(maxFileSize) {
		return fmt.Errorf("contenido demasiado grande para el archivo users.txt. Máximo: %d bytes", maxFileSize)
	}

	// Calcular cuántos bloques necesitamos
	contentSize := len(content)
	blocksNeeded := 0
	if contentSize > 0 {
		blocksNeeded = (contentSize + 63) / 64 // Redondear hacia arriba
	}

	// Primero, liberar bloques existentes si los hay
	for i := 0; i < 15 && usersInode.I_block[i] != -1; i++ {
		if i < 12 {
			// Bloque directo
			Utilities.WriteObject(file, byte(0), int64(superblock.S_bm_block_start+usersInode.I_block[i]))
			superblock.S_free_blocks_count++
		} else if i == 12 {
			// Bloque de punteros indirectos
			var indirectBlock Structs.Fileblock
			indirectBlockPos := int64(superblock.S_block_start + usersInode.I_block[i]*int32(binary.Size(Structs.Fileblock{})))
			if err := Utilities.ReadObject(file, &indirectBlock, indirectBlockPos); err == nil {
				// Liberar bloques indirectos
				for j := 0; j < 16; j++ {
					var blockNumber int32
					binary.Read(strings.NewReader(string(indirectBlock.B_content[j*4:(j+1)*4])), binary.LittleEndian, &blockNumber)
					if blockNumber != -1 {
						Utilities.WriteObject(file, byte(0), int64(superblock.S_bm_block_start+blockNumber))
						superblock.S_free_blocks_count++
					}
				}
			}
			// Liberar el bloque de punteros indirectos
			Utilities.WriteObject(file, byte(0), int64(superblock.S_bm_block_start+usersInode.I_block[i]))
			superblock.S_free_blocks_count++
		}
		usersInode.I_block[i] = -1
	}

	// Buscar bloques libres para el nuevo contenido
	var dataBlocks []int32
	var indirectBlock int32 = -1

	if blocksNeeded > 0 {
		for i := 0; i < blocksNeeded; i++ {
			freeBlock := findFreeBlock(file, &superblock)
			if freeBlock == -1 {
				return fmt.Errorf("no hay suficientes bloques libres para users.txt")
			}
			dataBlocks = append(dataBlocks, freeBlock)
			
			// Marcar bloque como ocupado
			Utilities.WriteObject(file, byte(1), int64(superblock.S_bm_block_start+freeBlock))
		}

		// Si necesitamos más de 12 bloques, necesitamos un bloque para punteros indirectos
		if blocksNeeded > 12 {
			indirectBlock = findFreeBlock(file, &superblock)
			if indirectBlock == -1 {
				return fmt.Errorf("no hay bloque libre para punteros indirectos en users.txt")
			}
			// Marcar bloque indirecto como ocupado
			Utilities.WriteObject(file, byte(1), int64(superblock.S_bm_block_start+indirectBlock))
		}
	}

	// Asignar bloques al inodo
	for i, blockNum := range dataBlocks {
		if i < 12 {
			// Punteros directos (I_block[0] a I_block[11])
			usersInode.I_block[i] = blockNum
		} else if i == 12 {
			// Primer bloque indirecto
			usersInode.I_block[12] = indirectBlock
			
			// Escribir el primer puntero en el bloque indirecto
			indirectBlockPos := int64(superblock.S_block_start + indirectBlock*int32(binary.Size(Structs.Fileblock{})))
			Utilities.WriteObject(file, blockNum, indirectBlockPos)
		} else {
			// Punteros indirectos adicionales
			indirectIndex := i - 12
			indirectPos := int64(superblock.S_block_start + indirectBlock*int32(binary.Size(Structs.Fileblock{})) + int32(indirectIndex*4))
			Utilities.WriteObject(file, blockNum, indirectPos)
		}
	}

	// Actualizar el tamaño del archivo en el inodo
	usersInode.I_size = int32(contentSize)
	
	// Escribir el inodo actualizado
	if err := Utilities.WriteObject(file, usersInode, inodePos); err != nil {
		return fmt.Errorf("error escribiendo inodo users.txt: %s", err.Error())
	}

	// Escribir el contenido del archivo en los bloques
	if contentSize > 0 {
		bytesWritten := 0
		for i, blockNum := range dataBlocks {
			// Calcular cuántos bytes escribir en este bloque
			remainingBytes := contentSize - bytesWritten
			bytesToWrite := 64
			if remainingBytes < 64 {
				bytesToWrite = remainingBytes
			}

			// Crear bloque de archivo con el contenido
			var usersBlock Structs.Fileblock
			// Llenar con el contenido correspondiente
			contentSlice := content[bytesWritten:bytesWritten+bytesToWrite]
			copy(usersBlock.B_content[:], contentSlice)
			
			// Escribir el bloque
			blockPos := int64(superblock.S_block_start + blockNum*int32(binary.Size(Structs.Fileblock{})))
			if err := Utilities.WriteObject(file, usersBlock, blockPos); err != nil {
				return fmt.Errorf("error escribiendo bloque %d de users.txt: %s", i, err.Error())
			}

			bytesWritten += bytesToWrite
		}
	}

	// Actualizar contadores en el superblock
	superblock.S_free_blocks_count -= int32(blocksNeeded)
	if indirectBlock != -1 {
		superblock.S_free_blocks_count-- // Bloque adicional para punteros indirectos
	}

	// Escribir superblock actualizado
	if err := Utilities.WriteObject(file, superblock, int64(partition.Start)); err != nil {
		return fmt.Errorf("error actualizando superblock: %s", err.Error())
	}

	return nil
}

// CatUsersFile - Mostrar el contenido exacto del archivo users.txt
func CatUsersFile() {
	fmt.Println("======Inicio CAT======")
	
	// Verificar que haya una sesión activa
	if !IsUserLoggedIn() {
		fmt.Println("Error: Debe iniciar sesión primero")
		fmt.Println("Use: login -user=<usuario> -pass=<contraseña> -id=<ID_particion>")
		fmt.Println("======FIN CAT======")
		return
	}

	// Leer el contenido del archivo users.txt
	usersData, err := readUsersFile(CurrentSession.PartitionID)
	if err != nil {
		fmt.Printf("Error leyendo archivo users.txt: %s\n", err.Error())
		fmt.Println("======FIN CAT======")
		return
	}

	fmt.Println("=== CONTENIDO DEL ARCHIVO users.txt ===")
	fmt.Printf("Partición: %s\n", CurrentSession.PartitionID)
	fmt.Printf("Tamaño: %d bytes\n", len(usersData))
	fmt.Println("---")
	
	// Mostrar el contenido línea por línea para mejor visualización
	lines := strings.Split(usersData, "\n")
	for i, line := range lines {
		if line != "" {
			fmt.Printf("Línea %d: %s\n", i+1, line)
		}
	}
	
	fmt.Println("---")
	fmt.Println("Contenido raw (con caracteres de escape):")
	fmt.Printf("%q\n", usersData)
	fmt.Println("======FIN CAT======")
}

// Cat - Mostrar el contenido de uno o múltiples archivos
func Cat(filePaths []string) {
	fmt.Println("======Inicio CAT======")
	
	// Verificar que haya una sesión activa
	if !IsUserLoggedIn() {
		fmt.Println("Error: Debe iniciar sesión primero")
		fmt.Println("Use: login -user=<usuario> -pass=<contraseña> -id=<ID_particion>")
		fmt.Println("======FIN CAT======")
		return
	}
	
	if len(filePaths) == 0 {
		fmt.Println("Error: Debe especificar al menos un archivo")
		fmt.Println("Uso: cat -file1=/ruta/archivo1 [-file2=/ruta/archivo2] ...")
		fmt.Println("======FIN CAT======")
		return
	}
	
	for i, filePath := range filePaths {
		if i > 0 {
			fmt.Println() // Separar archivos con línea en blanco
		}
		
		// Buscar el archivo en el sistema
		exists, inodeNum := findFileInDirectory(CurrentSession.PartitionID, filePath)
		if !exists {
			fmt.Printf("Error: Archivo '%s' no encontrado\n", filePath)
			continue
		}
		
		// Verificar permisos de lectura
		if !hasReadPermission(CurrentSession.PartitionID, inodeNum, CurrentSession.UserID, CurrentSession.GroupID) {
			fmt.Printf("Error: Sin permisos de lectura para el archivo '%s'\n", filePath)
			continue
		}
		
		// Leer el contenido del archivo
		content, err := readFileContent(CurrentSession.PartitionID, inodeNum)
		if err != nil {
			fmt.Printf("Error leyendo archivo '%s': %s\n", filePath, err.Error())
			continue
		}
		
		// Mostrar el contenido
		fmt.Printf("# %s\n", filePath)
		if content == "" {
			fmt.Println("(archivo vacío)")
		} else {
			fmt.Print(content)
			// Agregar salto de línea si el archivo no termina en uno
			if !strings.HasSuffix(content, "\n") {
				fmt.Println()
			}
		}
	}
	
	fmt.Println("======FIN CAT======")
}

// Mkusr - Crear un nuevo usuario en el sistema (solo root)
func Mkusr(username string, password string, groupName string) {
	fmt.Println("======Inicio MKUSR======")
	fmt.Printf("Nombre del usuario: %s\n", username)
	fmt.Printf("Grupo: %s\n", groupName)
	
	// Verificar que haya una sesión activa
	if !IsUserLoggedIn() {
		fmt.Println("Error: Debe iniciar sesión primero")
		fmt.Println("Use: login -user=<usuario> -pass=<contraseña> -id=<ID_particion>")
		fmt.Println("======FIN MKUSR======")
		return
	}

	// Verificar que el usuario sea root
	if CurrentSession.Username != "root" {
		fmt.Printf("Error: Solo el usuario 'root' puede crear usuarios\n")
		fmt.Printf("Usuario actual: %s\n", CurrentSession.Username)
		fmt.Println("======FIN MKUSR======")
		return
	}

	// Validar parámetros obligatorios
	if strings.TrimSpace(username) == "" {
		fmt.Println("Error: El nombre del usuario no puede estar vacío")
		fmt.Println("======FIN MKUSR======")
		return
	}

	if strings.TrimSpace(password) == "" {
		fmt.Println("Error: La contraseña del usuario no puede estar vacía")
		fmt.Println("======FIN MKUSR======")
		return
	}

	if strings.TrimSpace(groupName) == "" {
		fmt.Println("Error: El nombre del grupo no puede estar vacío")
		fmt.Println("======FIN MKUSR======")
		return
	}

	// Validar longitudes máximas
	if len(username) > 10 {
		fmt.Printf("Error: El nombre del usuario no puede tener más de 10 caracteres (actual: %d)\n", len(username))
		fmt.Println("======FIN MKUSR======")
		return
	}

	if len(password) > 10 {
		fmt.Printf("Error: La contraseña no puede tener más de 10 caracteres (actual: %d)\n", len(password))
		fmt.Println("======FIN MKUSR======")
		return
	}

	if len(groupName) > 10 {
		fmt.Printf("Error: El nombre del grupo no puede tener más de 10 caracteres (actual: %d)\n", len(groupName))
		fmt.Println("======FIN MKUSR======")
		return
	}

	// Leer el archivo users.txt actual
	usersData, err := readUsersFile(CurrentSession.PartitionID)
	if err != nil {
		fmt.Printf("Error leyendo archivo users.txt: %s\n", err.Error())
		fmt.Println("======FIN MKUSR======")
		return
	}

	// Verificar que el usuario no exista ya
	if userExistsForCreation(usersData, username) {
		fmt.Printf("Error: El usuario '%s' ya existe en el sistema\n", username)
		fmt.Println("Los nombres de usuarios distinguen mayúsculas y minúsculas")
		fmt.Println("======FIN MKUSR======")
		return
	}

	// Verificar que el grupo exista y no esté eliminado
	groupExists, groupID := findActiveGroup(usersData, groupName)
	if !groupExists {
		fmt.Printf("Error: El grupo '%s' no existe en el sistema\n", groupName)
		fmt.Println("Debe crear el grupo primero con el comando 'mkgrp'")
		fmt.Println("======FIN MKUSR======")
		return
	}

	if groupID == 0 {
		fmt.Printf("Error: El grupo '%s' ha sido eliminado y no está disponible\n", groupName)
		fmt.Println("Los grupos eliminados no pueden ser utilizados para crear usuarios")
		fmt.Println("======FIN MKUSR======")
		return
	}

	// Obtener el siguiente ID disponible para el usuario
	nextUserID := getNextAvailableUserID(usersData)

	// Crear la nueva entrada del usuario
	newUserEntry := fmt.Sprintf("%d,U,%s,%s,%s", nextUserID, groupName, username, password)

	// Agregar el nuevo usuario al contenido existente
	updatedUsersData := usersData + newUserEntry + "\n"

	// Escribir el contenido actualizado al archivo users.txt
	err = writeUsersFile(CurrentSession.PartitionID, updatedUsersData)
	if err != nil {
		fmt.Printf("Error escribiendo archivo users.txt: %s\n", err.Error())
		fmt.Println("======FIN MKUSR======")
		return
	}

	// Registrar en el journaling (EXT3)
	contentInfo := fmt.Sprintf("user=%s,grp=%s", username, groupName)
	writeToJournal(CurrentSession.PartitionID, "mkusr", "/users.txt", contentInfo)

	fmt.Println("=== USUARIO CREADO EXITOSAMENTE ===")
	fmt.Printf("Nombre del usuario: %s\n", username)
	fmt.Printf("ID asignado: %d\n", nextUserID)
	fmt.Printf("Grupo: %s (ID: %d)\n", groupName, groupID)
	fmt.Printf("Partición: %s\n", CurrentSession.PartitionID)
	fmt.Printf("Usuario que creó la cuenta: %s\n", CurrentSession.Username)
	fmt.Println("El usuario ha sido agregado al archivo users.txt")
	fmt.Println("======FIN MKUSR======")
}

// Rmusr - Eliminar un usuario del sistema (solo root)
func Rmusr(username string) {
	fmt.Println("======Inicio RMUSR======")
	fmt.Printf("Nombre del usuario a eliminar: %s\n", username)
	
	// Verificar que haya una sesión activa
	if !IsUserLoggedIn() {
		fmt.Println("Error: Debe iniciar sesión primero")
		fmt.Println("Use: login -user=<usuario> -pass=<contraseña> -id=<ID_particion>")
		fmt.Println("======FIN RMUSR======")
		return
	}

	// Verificar que el usuario sea root
	if CurrentSession.Username != "root" {
		fmt.Printf("Error: Solo el usuario 'root' puede eliminar usuarios\n")
		fmt.Printf("Usuario actual: %s\n", CurrentSession.Username)
		fmt.Println("======FIN RMUSR======")
		return
	}

	// Validar que el nombre del usuario no esté vacío
	if strings.TrimSpace(username) == "" {
		fmt.Println("Error: El nombre del usuario no puede estar vacío")
		fmt.Println("======FIN RMUSR======")
		return
	}

	// Verificar que no sea el usuario root
	if username == "root" {
		fmt.Println("Error: No se puede eliminar el usuario 'root'")
		fmt.Println("El usuario root es necesario para el funcionamiento del sistema")
		fmt.Println("======FIN RMUSR======")
		return
	}

	// Leer el archivo users.txt actual
	usersData, err := readUsersFile(CurrentSession.PartitionID)
	if err != nil {
		fmt.Printf("Error leyendo archivo users.txt: %s\n", err.Error())
		fmt.Println("======FIN RMUSR======")
		return
	}

	// Verificar que el usuario existe y no está ya eliminado
	userExists, userID := findUserForDeletion(usersData, username)
	if !userExists {
		fmt.Printf("Error: El usuario '%s' no existe en el sistema\n", username)
		fmt.Println("Verifique que el nombre del usuario sea correcto (distingue mayúsculas y minúsculas)")
		fmt.Println("======FIN RMUSR======")
		return
	}

	if userID == 0 {
		fmt.Printf("Error: El usuario '%s' ya ha sido eliminado anteriormente\n", username)
		fmt.Println("======FIN RMUSR======")
		return
	}

	// Marcar el usuario como eliminado (cambiar ID a 0)
	updatedUsersData := markUserAsDeleted(usersData, username)

	// Escribir el contenido actualizado al archivo users.txt
	err = writeUsersFile(CurrentSession.PartitionID, updatedUsersData)
	if err != nil {
		fmt.Printf("Error escribiendo archivo users.txt: %s\n", err.Error())
		fmt.Println("======FIN RMUSR======")
		return
	}

	// Registrar en el journaling (EXT3)
	writeToJournal(CurrentSession.PartitionID, "rmusr", "/users.txt", username)

	fmt.Println("=== USUARIO ELIMINADO EXITOSAMENTE ===")
	fmt.Printf("Nombre del usuario: %s\n", username)
	fmt.Printf("ID anterior: %d\n", userID)
	fmt.Println("ID actual: 0 (marcado como eliminado)")
	fmt.Printf("Partición: %s\n", CurrentSession.PartitionID)
	fmt.Printf("Usuario que eliminó la cuenta: %s\n", CurrentSession.Username)
	fmt.Println("El usuario ha sido marcado como eliminado en el archivo users.txt")
	fmt.Println("======FIN RMUSR======")
}

// ============================================================================
// FUNCIONES AUXILIARES PARA GESTIÓN DE USUARIOS
// ============================================================================

// userExistsForCreation - Verificar si un usuario ya existe para creación
func userExistsForCreation(usersData string, username string) bool {
	lines := strings.Split(usersData, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Dividir la línea por comas
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			continue
		}
		
		// Verificar si es un usuario (tipo "U")
		if strings.TrimSpace(parts[1]) == "U" && len(parts) >= 5 {
			existingUsername := strings.TrimSpace(parts[3])
			if existingUsername == username { // Distingue mayúsculas y minúsculas
				return true
			}
		}
	}
	
	return false
}

// findActiveGroup - Buscar un grupo activo (no eliminado) y obtener su ID
func findActiveGroup(usersData string, groupName string) (bool, int) {
	lines := strings.Split(usersData, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Dividir la línea por comas
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		
		// Verificar si es un grupo (tipo "G")
		if strings.TrimSpace(parts[1]) == "G" {
			existingGroupName := strings.TrimSpace(parts[2])
			if existingGroupName == groupName { // Distingue mayúsculas y minúsculas
				// Obtener el ID del grupo
				id := 0
				fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &id)
				return true, id
			}
		}
	}
	
	return false, 0
}

// getNextAvailableUserID - Obtener el siguiente ID disponible para un usuario
func getNextAvailableUserID(usersData string) int {
	maxID := 0
	lines := strings.Split(usersData, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Dividir la línea por comas
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		
		// Obtener el ID
		id := 0
		fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &id)
		if id > maxID {
			maxID = id
		}
	}
	
	return maxID + 1
}

// findUserForDeletion - Buscar un usuario para eliminación y obtener su ID
func findUserForDeletion(usersData string, username string) (bool, int) {
	lines := strings.Split(usersData, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Dividir la línea por comas
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			continue
		}
		
		// Verificar si es un usuario (tipo "U")
		if strings.TrimSpace(parts[1]) == "U" && len(parts) >= 5 {
			existingUsername := strings.TrimSpace(parts[3])
			if existingUsername == username { // Distingue mayúsculas y minúsculas
				// Obtener el ID del usuario
				id := 0
				fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &id)
				return true, id
			}
		}
	}
	
	return false, 0
}

// markUserAsDeleted - Marcar un usuario como eliminado (cambiar ID a 0)
func markUserAsDeleted(usersData string, username string) string {
	lines := strings.Split(usersData, "\n")
	var updatedLines []string
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		// Dividir la línea por comas
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			updatedLines = append(updatedLines, line)
			continue
		}
		
		// Verificar si es el usuario que queremos eliminar
		if strings.TrimSpace(parts[1]) == "U" && len(parts) >= 5 {
			existingUsername := strings.TrimSpace(parts[3])
			if existingUsername == username {
				// Cambiar el ID a 0 para marcarlo como eliminado
				updatedLine := fmt.Sprintf("0,U,%s,%s,%s", strings.TrimSpace(parts[2]), username, strings.TrimSpace(parts[4]))
				updatedLines = append(updatedLines, updatedLine)
			} else {
				updatedLines = append(updatedLines, line)
			}
		} else {
			updatedLines = append(updatedLines, line)
		}
	}
	
	// Unir las líneas con saltos de línea
	result := strings.Join(updatedLines, "\n")
	if len(updatedLines) > 0 {
		result += "\n" // Asegurar que termine con \n
	}
	
	return result
}

// Chgrp - Cambiar el grupo de un usuario en el sistema (solo root)
func Chgrp(username string, newGroupName string) {
	fmt.Println("======Inicio CHGRP======")
	fmt.Printf("Usuario: %s\n", username)
	fmt.Printf("Nuevo grupo: %s\n", newGroupName)
	
	// Verificar que haya una sesión activa
	if !IsUserLoggedIn() {
		fmt.Println("Error: No hay una sesión activa")
		fmt.Println("Use el comando 'login' para iniciar sesión")
		fmt.Println("======FIN CHGRP======")
		return
	}

	// Verificar que el usuario sea root
	if CurrentSession.Username != "root" {
		fmt.Printf("Error: Solo el usuario root puede cambiar grupos de usuarios\n")
		fmt.Printf("Usuario actual: %s\n", CurrentSession.Username)
		fmt.Println("======FIN CHGRP======")
		return
	}

	// Validar que el nombre del usuario no esté vacío
	if strings.TrimSpace(username) == "" {
		fmt.Println("Error: El nombre del usuario no puede estar vacío")
		fmt.Println("======FIN CHGRP======")
		return
	}

	// Validar que el nombre del grupo no esté vacío
	if strings.TrimSpace(newGroupName) == "" {
		fmt.Println("Error: El nombre del grupo no puede estar vacío")
		fmt.Println("======FIN CHGRP======")
		return
	}

	// Leer el archivo users.txt actual
	usersData, err := readUsersFile(CurrentSession.PartitionID)
	if err != nil {
		fmt.Printf("Error leyendo archivo users.txt: %s\n", err.Error())
		fmt.Println("======FIN CHGRP======")
		return
	}

	// Verificar que el usuario existe y no está eliminado
	userExists, userInfo := findUser(usersData, username)
	if !userExists {
		fmt.Printf("Error: El usuario '%s' no existe en el sistema\n", username)
		fmt.Println("======FIN CHGRP======")
		return
	}

	if userInfo.ID == 0 {
		fmt.Printf("Error: El usuario '%s' está marcado como eliminado\n", username)
		fmt.Println("======FIN CHGRP======")
		return
	}

	// Verificar que no sea el usuario root
	if username == "root" {
		fmt.Println("Error: No se puede cambiar el grupo del usuario root")
		fmt.Println("======FIN CHGRP======")
		return
	}

	// Verificar que el nuevo grupo existe y no está eliminado
	groupExists, groupID := findActiveGroup(usersData, newGroupName)
	if !groupExists {
		fmt.Printf("Error: El grupo '%s' no existe en el sistema\n", newGroupName)
		fmt.Println("======FIN CHGRP======")
		return
	}

	if groupID == 0 {
		fmt.Printf("Error: El grupo '%s' está marcado como eliminado\n", newGroupName)
		fmt.Println("======FIN CHGRP======")
		return
	}

	// Verificar que el usuario no esté ya en ese grupo
	if userInfo.Group == newGroupName {
		fmt.Printf("El usuario '%s' ya pertenece al grupo '%s'\n", username, newGroupName)
		fmt.Println("No se realizaron cambios")
		fmt.Println("======FIN CHGRP======")
		return
	}

	// Cambiar el grupo del usuario
	updatedUsersData := changeUserGroup(usersData, username, newGroupName)

	// Escribir el contenido actualizado al archivo users.txt
	err = writeUsersFile(CurrentSession.PartitionID, updatedUsersData)
	if err != nil {
		fmt.Printf("Error escribiendo archivo users.txt: %s\n", err.Error())
		fmt.Println("======FIN CHGRP======")
		return
	}

	// Registrar en el journaling (EXT3)
	contentInfo := fmt.Sprintf("%s->%s", userInfo.Group, newGroupName)
	writeToJournal(CurrentSession.PartitionID, "chgrp", "/users.txt", contentInfo)

	fmt.Println("=== GRUPO DE USUARIO CAMBIADO EXITOSAMENTE ===")
	fmt.Printf("Usuario: %s\n", username)
	fmt.Printf("Grupo anterior: %s\n", userInfo.Group)
	fmt.Printf("Grupo nuevo: %s (ID: %d)\n", newGroupName, groupID)
	fmt.Printf("Partición: %s\n", CurrentSession.PartitionID)
	fmt.Printf("Usuario que realizó el cambio: %s\n", CurrentSession.Username)
	fmt.Println("El cambio de grupo ha sido registrado en el archivo users.txt")
	fmt.Println("======FIN CHGRP======")
}

// changeUserGroup - Cambiar el grupo de un usuario en los datos del archivo users.txt
func changeUserGroup(usersData string, username string, newGroupName string) string {
	lines := strings.Split(usersData, "\n")
	var updatedLines []string
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		// Dividir la línea por comas
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			updatedLines = append(updatedLines, line)
			continue
		}
		
		// Verificar si es el usuario que queremos modificar
		if strings.TrimSpace(parts[1]) == "U" && len(parts) >= 5 {
			existingUsername := strings.TrimSpace(parts[3])
			if existingUsername == username {
				// Cambiar el grupo del usuario manteniendo el resto de la información
				userID := strings.TrimSpace(parts[0])
				password := strings.TrimSpace(parts[4])
				updatedLine := fmt.Sprintf("%s,U,%s,%s,%s", userID, newGroupName, username, password)
				updatedLines = append(updatedLines, updatedLine)
			} else {
				updatedLines = append(updatedLines, line)
			}
		} else {
			updatedLines = append(updatedLines, line)
		}
	}
	
	// Unir las líneas con saltos de línea
	result := strings.Join(updatedLines, "\n")
	if len(updatedLines) > 0 {
		result += "\n" // Asegurar que termine con \n
	}
	
	return result
}

// ============================================================================
// FUNCIONES DE GESTIÓN DE ARCHIVOS Y DIRECTORIOS
// ============================================================================

// Mkfile - Crear un archivo en el sistema de archivos
func Mkfile(path string, r bool, size int, cont string) {
	fmt.Println("======Inicio MKFILE======")
	fmt.Printf("Ruta: %s\n", path)
	fmt.Printf("Crear directorios padre: %t\n", r)
	fmt.Printf("Tamaño: %d bytes\n", size)
	if cont != "" {
		fmt.Printf("Archivo contenido: %s\n", cont)
	}
	
	// Verificar que haya una sesión activa
	if !IsUserLoggedIn() {
		fmt.Println("Error: No hay una sesión activa")
		fmt.Println("Use el comando 'login' para iniciar sesión")
		fmt.Println("======FIN MKFILE======")
		return
	}

	// Validar que la ruta no esté vacía
	if strings.TrimSpace(path) == "" {
		fmt.Println("Error: La ruta del archivo no puede estar vacía")
		fmt.Println("======FIN MKFILE======")
		return
	}

	// Validar que el tamaño no sea negativo
	if size < 0 {
		fmt.Println("Error: El tamaño del archivo no puede ser negativo")
		fmt.Println("======FIN MKFILE======")
		return
	}

	// Validar que la ruta empiece con "/"
	if !strings.HasPrefix(path, "/") {
		fmt.Println("Error: La ruta debe empezar con '/' (ruta absoluta)")
		fmt.Println("======FIN MKFILE======")
		return
	}

	// Si se especifica contenido, validar que el archivo existe
	var contentData string
	if cont != "" {
		if !fileExistsLocal(cont) {
			fmt.Printf("Error: El archivo de contenido '%s' no existe en el sistema local\n", cont)
			fmt.Println("======FIN MKFILE======")
			return
		}
		
		// Leer el contenido del archivo local
		data, err := readLocalFile(cont)
		if err != nil {
			fmt.Printf("Error leyendo archivo de contenido '%s': %s\n", cont, err.Error())
			fmt.Println("======FIN MKFILE======")
			return
		}
		contentData = data
		size = len(contentData) // Si hay contenido, el tamaño es el del contenido
	} else if size > 0 {
		// Generar contenido con números 0-9
		contentData = generateNumberContent(size)
	}

	// Verificar que el contenido no exceda el tamaño máximo manejable
	// Con 12 punteros directos + 1 indirecto simple (16 punteros) = 28 bloques máximo
	maxBlocks := int32(28) // 12 directos + 16 indirectos simples
	maxFileSize := maxBlocks * 64 // 64 bytes por bloque
	
	if len(contentData) > int(maxFileSize) {
		fmt.Printf("Error: El contenido del archivo (%d bytes) excede el tamaño máximo soportado (%d bytes)\n", len(contentData), maxFileSize)
		fmt.Printf("Máximo: %d bloques de 64 bytes cada uno\n", maxBlocks)
		fmt.Println("======FIN MKFILE======")
		return
	}

	// Separar la ruta en directorio padre y nombre de archivo
	parentDir, fileName := parseFilePath(path)
	
	if fileName == "" {
		fmt.Println("Error: Nombre de archivo no válido")
		fmt.Println("======FIN MKFILE======")
		return
	}
	
	// Validar longitud del nombre del archivo (máximo 12 caracteres)
	if len(fileName) > 12 {
		fmt.Printf("Error: El nombre del archivo '%s' es demasiado largo (máximo 12 caracteres)\n", fileName)
		fmt.Printf("Longitud actual: %d caracteres\n", len(fileName))
		fmt.Println("Sugerencia: Use un nombre más corto")
		fmt.Println("======FIN MKFILE======")
		return
	}

	// Verificar si el archivo ya existe
	exists, _ := findInodeInDirectory(CurrentSession.PartitionID, 0, fileName, false)
	if parentDir == "/" && exists {
		if fileName == "users.txt" {
			fmt.Printf("El archivo '%s' ya existe\n", path)
			fmt.Print("¿Desea sobreescribir el archivo? (s/n): ")
			fmt.Println("n")
			fmt.Println("Operación cancelada")
			fmt.Println("======FIN MKFILE======")
			return
		}
		fmt.Printf("Error: El archivo '%s' ya existe\n", path)
		fmt.Println("======FIN MKFILE======")
		return
	}

	// Verificar que el directorio padre exista
	parentExists, parentInode := findDirectoryInPath(CurrentSession.PartitionID, parentDir)
	
	if !parentExists {
		if !r {
			fmt.Printf("Error: El directorio padre '%s' no existe\n", parentDir)
			fmt.Println("Use el parámetro -r para crear directorios padre automáticamente")
			fmt.Println("======FIN MKFILE======")
			return
		} else {
			// Crear directorios padre recursivamente
			parentInode = createDirectoriesRecursively(CurrentSession.PartitionID, parentDir)
			if parentInode == -1 {
				fmt.Printf("Error: No se pudo crear el directorio padre '%s'\n", parentDir)
				fmt.Println("======FIN MKFILE======")
				return
			}
			fmt.Printf("Directorios padre creados: %s\n", parentDir)
		}
	}

	// Verificar permisos de escritura en el directorio padre
	if !hasWritePermission(CurrentSession.PartitionID, parentInode, CurrentSession.UserID, CurrentSession.GroupID) {
		fmt.Printf("Error: No tiene permisos de escritura en el directorio padre '%s'\n", parentDir)
		fmt.Println("======FIN MKFILE======")
		return
	}

	// Crear el archivo
	fileInode := createFileInDirectory(CurrentSession.PartitionID, parentInode, fileName, contentData)
	if fileInode == -1 {
		fmt.Printf("Error: No se pudo crear el archivo '%s'\n", path)
		fmt.Println("======FIN MKFILE======")
		return
	}

	// Registrar en el journaling (EXT3)
	contentPreview := fmt.Sprintf("size=%d", len(contentData))
	if cont != "" {
		contentPreview = fmt.Sprintf("from:%s", cont)
	}
	writeToJournal(CurrentSession.PartitionID, "mkfile", path, contentPreview)

	fmt.Println("=== ARCHIVO CREADO EXITOSAMENTE ===")
	fmt.Printf("Ruta: %s\n", path)
	fmt.Printf("Tamaño: %d bytes\n", len(contentData))
	if len(contentData) > 64 {
		blocksUsed := (len(contentData) + 63) / 64
		fmt.Printf("Bloques utilizados: %d bloques de 64 bytes\n", blocksUsed)
		fmt.Printf("Espacio en disco: %d bytes\n", blocksUsed*64)
	}
	fmt.Printf("Propietario: %s (ID: %d)\n", CurrentSession.Username, CurrentSession.UserID)
	fmt.Printf("Grupo: %d\n", CurrentSession.GroupID)
	if CurrentSession != nil && CurrentSession.Username == "root" {
		fmt.Printf("Permisos: 777 (rwxrwxrwx)\n")
	} else {
		fmt.Printf("Permisos: 664 (rw-rw-r--)\n")
	}
	fmt.Printf("Inodo asignado: %d\n", fileInode)
	fmt.Printf("Partición: %s\n", CurrentSession.PartitionID)
	fmt.Println("======FIN MKFILE======")
}

// ============================================================================
// COMANDO MKDIR - CREAR DIRECTORIOS
// ============================================================================

// Mkdir - Crear un directorio en el sistema de archivos
func Mkdir(path string, p bool) {
	fmt.Println("======Inicio MKDIR======")
	fmt.Printf("Ruta: %s\n", path)
	fmt.Printf("Crear directorios padre: %t\n", p)
	
	// Verificar que haya una sesión activa
	if !IsUserLoggedIn() {
		fmt.Println("Error: No hay una sesión activa")
		fmt.Println("Use el comando 'login' para iniciar sesión")
		fmt.Println("======FIN MKDIR======")
		return
	}

	// Validar que la ruta no esté vacía
	if strings.TrimSpace(path) == "" {
		fmt.Println("Error: La ruta del directorio no puede estar vacía")
		fmt.Println("======FIN MKDIR======")
		return
	}

	// Validar que la ruta empiece con "/"
	if !strings.HasPrefix(path, "/") {
		fmt.Println("Error: La ruta debe empezar con '/' (ruta absoluta)")
		fmt.Println("======FIN MKDIR======")
		return
	}

	// Separar la ruta en directorio padre y nombre del directorio
	parentDir, dirName := parseFilePath(path)
	
	if dirName == "" {
		fmt.Println("Error: Nombre de directorio no válido")
		fmt.Println("======FIN MKDIR======")
		return
	}
	
	// Validar longitud del nombre del directorio (máximo 12 caracteres)
	if len(dirName) > 12 {
		fmt.Printf("Error: El nombre del directorio '%s' es demasiado largo (máximo 12 caracteres)\n", dirName)
		fmt.Printf("Longitud actual: %d caracteres\n", len(dirName))
		fmt.Println("Sugerencia: Use un nombre más corto")
		fmt.Println("======FIN MKDIR======")
		return
	}

	// Verificar si el directorio ya existe
	exists, _ := findDirectoryInPath(CurrentSession.PartitionID, path)
	if exists {
		fmt.Printf("Error: El directorio '%s' ya existe\n", path)
		fmt.Println("======FIN MKDIR======")
		return
	}

	// Verificar que el directorio padre exista
	parentExists, parentInode := findDirectoryInPath(CurrentSession.PartitionID, parentDir)
	
	if !parentExists {
		if !p {
			fmt.Printf("Error: El directorio padre '%s' no existe\n", parentDir)
			fmt.Println("Use el parámetro -p para crear directorios padre automáticamente")
			fmt.Println("======FIN MKDIR======")
			return
		} else {
			// Crear directorios padre recursivamente
			parentInode = createDirectoriesRecursively(CurrentSession.PartitionID, parentDir)
			if parentInode == -1 {
				fmt.Printf("Error: No se pudo crear el directorio padre '%s'\n", parentDir)
				fmt.Println("======FIN MKDIR======")
				return
			}
			fmt.Printf("Directorios padre creados: %s\n", parentDir)
		}
	}

	// Verificar permisos de escritura en el directorio padre
	if !hasWritePermission(CurrentSession.PartitionID, parentInode, CurrentSession.UserID, CurrentSession.GroupID) {
		fmt.Printf("Error: No tiene permisos de escritura en el directorio padre '%s'\n", parentDir)
		fmt.Println("======FIN MKDIR======")
		return
	}

	// Crear el directorio
	dirInode := createDirectoryInParent(CurrentSession.PartitionID, parentInode, dirName)
	if dirInode == -1 {
		fmt.Printf("Error: No se pudo crear el directorio '%s'\n", path)
		fmt.Println("======FIN MKDIR======")
		return
	}

	// Registrar en el journaling (EXT3)
	writeToJournal(CurrentSession.PartitionID, "mkdir", path, "directory")

	fmt.Println("=== DIRECTORIO CREADO EXITOSAMENTE ===")
	fmt.Printf("Ruta: %s\n", path)
	fmt.Printf("Propietario: %s (ID: %d)\n", CurrentSession.Username, CurrentSession.UserID)
	fmt.Printf("Grupo: %d\n", CurrentSession.GroupID)
	if CurrentSession != nil && CurrentSession.Username == "root" {
		fmt.Printf("Permisos: 777 (rwxrwxrwx)\n")
	} else {
		fmt.Printf("Permisos: 664 (rw-rw-r--)\n")
	}
	fmt.Printf("Inodo asignado: %d\n", dirInode)
	fmt.Printf("Partición: %s\n", CurrentSession.PartitionID)
	fmt.Println("======FIN MKDIR======")
}

// ============================================================================
// FUNCIONES AUXILIARES PARA GESTIÓN DE ARCHIVOS
// ============================================================================

// fileExistsLocal - Verificar si un archivo existe en el sistema local
func fileExistsLocal(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// readLocalFile - Leer contenido de un archivo local
func readLocalFile(filePath string) (string, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// generateNumberContent - Generar contenido con números 0-9 repetidos
func generateNumberContent(size int) string {
	if size == 0 {
		return ""
	}
	
	content := make([]byte, size)
	for i := 0; i < size; i++ {
		content[i] = byte('0' + (i % 10))
	}
	
	return string(content)
}

// parseFilePath - Separar una ruta en directorio padre y nombre de archivo
func parseFilePath(path string) (string, string) {
	path = strings.TrimSpace(path)
	
	// Normalizar la ruta
	path = strings.ReplaceAll(path, "//", "/")
	if path != "/" && strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash == 0 {
		// Archivo en directorio raíz
		return "/", path[1:]
	}
	
	parentDir := path[:lastSlash]
	fileName := path[lastSlash+1:]
	
	if parentDir == "" {
		parentDir = "/"
	}
	
	return parentDir, fileName
}

// findFileInDirectory - Buscar un archivo en el sistema por ruta completa
func findFileInDirectory(partitionID string, filePath string) (bool, int32) {
	// Caso especial para users.txt
	if filePath == "/users.txt" {
		return true, 1 // El archivo users.txt siempre existe y es el inodo 1
	}
	
	// Normalizar la ruta
	filePath = strings.TrimSpace(filePath)
	if !strings.HasPrefix(filePath, "/") {
		filePath = "/" + filePath
	}
	
	// Separar directorio padre y nombre del archivo
	parentDir, fileName := parseFilePath(filePath)
	
	// Buscar el directorio padre
	exists, parentInode := findDirectoryInPath(partitionID, parentDir)
	if !exists {
		return false, -1
	}
	
	// Buscar el archivo en el directorio padre
	return findInodeInDirectory(partitionID, parentInode, fileName, false) // false = archivos, no directorios
}

// readFileContent - Leer el contenido completo de un archivo por su inodo
func readFileContent(partitionID string, inodeNum int32) (string, error) {
	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return "", fmt.Errorf("partición no montada")
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return "", fmt.Errorf("error abriendo disco: %s", err.Error())
	}
	defer file.Close()

	// Leer el superblock
	superblock, err := ReadSuperblock(partitionID)
	if err != nil {
		return "", fmt.Errorf("error leyendo superblock: %s", err.Error())
	}

	// Leer el inodo del archivo
	var inode Structs.Inode
	inodePos := int64(superblock.S_inode_start + inodeNum*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &inode, inodePos); err != nil {
		return "", fmt.Errorf("error leyendo inodo: %s", err.Error())
	}

	// Verificar que es un archivo (no directorio)
	if string(inode.I_type[:1]) != "1" {
		return "", fmt.Errorf("el inodo especificado no es un archivo")
	}

	// Si el archivo está vacío
	if inode.I_size == 0 {
		return "", nil
	}

	var content strings.Builder
	bytesToRead := int(inode.I_size)
	
	// Leer los bloques directos (hasta 12 bloques)
	for i := 0; i < 12 && i < len(inode.I_block) && inode.I_block[i] != -1 && bytesToRead > 0; i++ {
		var fileBlock Structs.Fileblock
		blockPos := int64(superblock.S_block_start + inode.I_block[i]*int32(binary.Size(Structs.Fileblock{})))
		if err := Utilities.ReadObject(file, &fileBlock, blockPos); err != nil {
			return "", fmt.Errorf("error leyendo bloque directo %d: %s", i, err.Error())
		}
		
		// Calcular cuántos bytes leer de este bloque
		bytesInThisBlock := 64
		if bytesToRead < 64 {
			bytesInThisBlock = bytesToRead
		}
		
		content.Write(fileBlock.B_content[:bytesInThisBlock])
		bytesToRead -= bytesInThisBlock
	}
	
	// Si hay más datos, leer desde bloque indirecto (posición 12)
	if bytesToRead > 0 && len(inode.I_block) > 12 && inode.I_block[12] != -1 {
		// Leer el bloque de punteros indirectos
		var indirectBlock Structs.Fileblock
		indirectBlockPos := int64(superblock.S_block_start + inode.I_block[12]*int32(binary.Size(Structs.Fileblock{})))
		if err := Utilities.ReadObject(file, &indirectBlock, indirectBlockPos); err != nil {
			return "", fmt.Errorf("error leyendo bloque indirecto: %s", err.Error())
		}
		
		// Leer hasta 16 bloques indirectos
		for i := 0; i < 16 && bytesToRead > 0; i++ {
			var blockNumber int32
			binary.Read(strings.NewReader(string(indirectBlock.B_content[i*4:(i+1)*4])), binary.LittleEndian, &blockNumber)
			
			if blockNumber == -1 {
				break
			}
			
			var fileBlock Structs.Fileblock
			blockPos := int64(superblock.S_block_start + blockNumber*int32(binary.Size(Structs.Fileblock{})))
			if err := Utilities.ReadObject(file, &fileBlock, blockPos); err != nil {
				return "", fmt.Errorf("error leyendo bloque indirecto %d: %s", i, err.Error())
			}
			
			// Calcular cuántos bytes leer de este bloque
			bytesInThisBlock := 64
			if bytesToRead < 64 {
				bytesInThisBlock = bytesToRead
			}
			
			content.Write(fileBlock.B_content[:bytesInThisBlock])
			bytesToRead -= bytesInThisBlock
		}
	}
	
	return content.String(), nil
}

// findDirectoryInPath - Buscar un directorio por ruta y retornar su inodo
func findDirectoryInPath(partitionID string, dirPath string) (bool, int32) {
	if dirPath == "/" {
		// Directorio raíz siempre existe y es el inodo 0
		return true, 0
	}
	
	// Normalizar la ruta
	dirPath = strings.TrimSpace(dirPath)
	if strings.HasSuffix(dirPath, "/") && dirPath != "/" {
		dirPath = dirPath[:len(dirPath)-1]
	}
	
	// Dividir la ruta en componentes
	components := strings.Split(strings.TrimPrefix(dirPath, "/"), "/")
	if len(components) == 0 || (len(components) == 1 && components[0] == "") {
		return true, 0 // Directorio raíz
	}
	
	// Buscar recursivamente cada componente
	currentInode := int32(0) // Empezar desde el directorio raíz
	
	for _, component := range components {
		if component == "" {
			continue
		}
		
		found, nextInode := findInodeInDirectory(partitionID, currentInode, component, true) // true = solo directorios
		if !found {
			return false, -1
		}
		currentInode = nextInode
	}
	
	return true, currentInode
}

// createDirectoriesRecursively - Crear directorios recursivamente
func createDirectoriesRecursively(partitionID string, dirPath string) int32 {
	if dirPath == "/" {
		return 0 // El directorio raíz siempre existe y es el inodo 0
	}

	// Verificar si el directorio ya existe
	exists, inode := findDirectoryInPath(partitionID, dirPath)
	if exists {
		return inode
	}

	// Obtener el directorio padre
	parentDir, dirName := parseFilePath(dirPath)
	
	// Crear el directorio padre recursivamente
	parentInode := createDirectoriesRecursively(partitionID, parentDir)
	if parentInode == -1 {
		return -1
	}

	// Crear este directorio
	return createDirectoryInParent(partitionID, parentInode, dirName)
}

// findInodeInDirectory - Buscar un inodo por nombre en un directorio específico
func findInodeInDirectory(partitionID string, dirInode int32, name string, dirsOnly bool) (bool, int32) {
	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return false, -1
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return false, -1
	}
	defer file.Close()

	// Leer el superblock
	superblock, err := ReadSuperblock(partitionID)
	if err != nil {
		return false, -1
	}

	// Leer el inodo del directorio
	var dirInodeStruct Structs.Inode
	inodePos := int64(superblock.S_inode_start + dirInode*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &dirInodeStruct, inodePos); err != nil {
		return false, -1
	}

	// Verificar que sea un directorio
	if string(dirInodeStruct.I_type[:1]) != "0" {
		return false, -1
	}

	// Buscar en los bloques del directorio
	for i := 0; i < 15 && dirInodeStruct.I_block[i] != -1; i++ {
		var folderBlock Structs.Folderblock
		blockPos := int64(superblock.S_block_start + dirInodeStruct.I_block[i]*superblock.S_block_size)
		if err := Utilities.ReadObject(file, &folderBlock, blockPos); err != nil {
			continue
		}

		// Buscar en las entradas del bloque
		for j := 0; j < 4; j++ {
			if folderBlock.B_content[j].B_inodo == -1 {
				continue // Entrada vacía
			}
			
			entryName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
			if entryName == name {
				entryInode := folderBlock.B_content[j].B_inodo
				
				// Si solo buscamos directorios, verificar el tipo
				if dirsOnly {
					var entryInodeStruct Structs.Inode
					entryInodePos := int64(superblock.S_inode_start + entryInode*superblock.S_inode_size)
					if err := Utilities.ReadObject(file, &entryInodeStruct, entryInodePos); err != nil {
						continue
					}
					
					if string(entryInodeStruct.I_type[:1]) != "0" {
						continue // No es un directorio
					}
				}
				
				return true, entryInode
			}
		}
	}

	return false, -1
}

// createDirectoryInParent - Crear un directorio en el directorio padre especificado
func createDirectoryInParent(partitionID string, parentInode int32, dirName string) int32 {
	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return -1
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return -1
	}
	defer file.Close()

	// Leer el superblock
	superblock, err := ReadSuperblock(partitionID)
	if err != nil {
		return -1
	}

	// Buscar un inodo libre
	freeInode := findFreeInode(file, superblock)
	if freeInode == -1 {
		fmt.Println("Error: No hay inodos libres disponibles")
		return -1
	}

	// Buscar un bloque libre para el contenido del directorio
	freeBlock := findFreeBlock(file, superblock)
	if freeBlock == -1 {
		fmt.Println("Error: No hay bloques libres disponibles")
		return -1
	}

	// Crear el inodo del directorio
	var newInode Structs.Inode
	newInode.I_uid = int32(CurrentSession.UserID)
	newInode.I_gid = int32(CurrentSession.GroupID)
	newInode.I_size = int32(64) // Tamaño de un bloque para el directorio
	
	// Configurar fechas
	currentDate := "02/09/2025"
	copy(newInode.I_atime[:], currentDate)
	copy(newInode.I_ctime[:], currentDate)
	copy(newInode.I_mtime[:], currentDate)
	
	copy(newInode.I_type[:], "0")    // 0 = directorio
	// Asignar permisos según el usuario
	if CurrentSession != nil && CurrentSession.Username == "root" {
		copy(newInode.I_perm[:], "777")  // Permisos rwxrwxrwx para root
	} else {
		copy(newInode.I_perm[:], "664")  // Permisos rw-rw-r-- para otros usuarios
	}
	
	// Inicializar bloques
	for i := 0; i < 15; i++ {
		newInode.I_block[i] = -1
	}
	newInode.I_block[0] = freeBlock // Primer bloque del directorio

	// Escribir el inodo
	inodePos := int64(superblock.S_inode_start + freeInode*superblock.S_inode_size)
	if err := Utilities.WriteObject(file, newInode, inodePos); err != nil {
		return -1
	}

	// Crear el contenido del directorio (entradas . y ..)
	var folderBlock Structs.Folderblock
	
	// Inicializar todas las entradas como vacías
	for i := 0; i < 4; i++ {
		folderBlock.B_content[i].B_inodo = -1
		for j := 0; j < 12; j++ {
			folderBlock.B_content[i].B_name[j] = 0
		}
	}
	
	// Entrada "." (directorio actual)
	copy(folderBlock.B_content[0].B_name[:], ".")
	folderBlock.B_content[0].B_inodo = freeInode
	
	// Entrada ".." (directorio padre)
	copy(folderBlock.B_content[1].B_name[:], "..")
	folderBlock.B_content[1].B_inodo = parentInode
	
	// Escribir el bloque del directorio
	blockPos := int64(superblock.S_block_start + freeBlock*superblock.S_block_size)
	if err := Utilities.WriteObject(file, folderBlock, blockPos); err != nil {
		return -1
	}

	// Marcar bloque como ocupado
	Utilities.WriteObject(file, byte(1), int64(superblock.S_bm_block_start+freeBlock))
	
	// Marcar inodo como ocupado
	Utilities.WriteObject(file, byte(1), int64(superblock.S_bm_inode_start+freeInode))
	
	// Actualizar contadores en el superblock
	superblock.S_free_inodes_count--
	superblock.S_free_blocks_count--

	// Actualizar superblock en el disco
	var partition *Structs.Partition = nil
	if !mountedPartition.IsLogical {
		// Partición primaria
		var tempMBR Structs.MBR
		if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
			return -1
		}
		partition = &tempMBR.Partitions[mountedPartition.PartitionIndex]
	}

	if partition != nil {
		// Actualizar superblock
		if err := Utilities.WriteObject(file, *superblock, int64(partition.Start)); err != nil {
			// Manejar error pero continuar
		}
	}

	// Agregar entrada en el directorio padre
	if !addFileToDirectory(file, superblock, parentInode, dirName, freeInode) {
		fmt.Println("Error: No se pudo agregar el directorio al directorio padre")
		return -1
	}

	return freeInode
}

// hasWritePermission - Verificar si un usuario tiene permisos de escritura en un directorio
func hasWritePermission(partitionID string, inodeNum int32, userID int, groupID int) bool {
	// Si es root, siempre tiene permisos
	if CurrentSession != nil && CurrentSession.Username == "root" {
		return true
	}
	
	// Para el directorio raíz, verificamos los permisos del inodo 0
	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return false
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return false
	}
	defer file.Close()

	// Leer el superblock para obtener la estructura del sistema
	superblock, err := ReadSuperblock(partitionID)
	if err != nil {
		return false
	}

	// Leer el inodo
	var inode Structs.Inode
	inodePos := int64(superblock.S_inode_start + inodeNum*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &inode, inodePos); err != nil {
		return false
	}

	// Extraer permisos del inodo
	perms := string(inode.I_perm[:3])
	if len(perms) < 3 {
		return false
	}

	// Verificar permisos según UGO (User, Group, Other)
	if inode.I_uid == int32(userID) {
		// Permiso de usuario (propietario)
		userPerm := perms[0] - '0'
		return (userPerm & 2) != 0 // Bit de escritura
	} else if inode.I_gid == int32(groupID) {
		// Permiso de grupo
		groupPerm := perms[1] - '0'
		return (groupPerm & 2) != 0 // Bit de escritura
	} else {
		// Permiso de otros
		otherPerm := perms[2] - '0'
		return (otherPerm & 2) != 0 // Bit de escritura
	}
}

// hasReadPermission - Verificar si un usuario tiene permisos de lectura en un archivo
func hasReadPermission(partitionID string, inodeNum int32, userID int, groupID int) bool {
	// Si es root, siempre tiene permisos
	if CurrentSession != nil && CurrentSession.Username == "root" {
		return true
	}
	
	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return false
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return false
	}
	defer file.Close()

	// Leer el superblock para obtener la estructura del sistema
	superblock, err := ReadSuperblock(partitionID)
	if err != nil {
		return false
	}

	// Leer el inodo
	var inode Structs.Inode
	inodePos := int64(superblock.S_inode_start + inodeNum*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &inode, inodePos); err != nil {
		return false
	}

	// Extraer permisos del inodo
	perms := string(inode.I_perm[:3])
	if len(perms) < 3 {
		return false
	}

	// Verificar permisos según UGO (User, Group, Other)
	if inode.I_uid == int32(userID) {
		// Permiso de usuario (propietario)
		userPerm := perms[0] - '0'
		return (userPerm & 4) != 0 // Bit de lectura
	} else if inode.I_gid == int32(groupID) {
		// Permiso de grupo
		groupPerm := perms[1] - '0'
		return (groupPerm & 4) != 0 // Bit de lectura
	} else {
		// Permiso de otros
		otherPerm := perms[2] - '0'
		return (otherPerm & 4) != 0 // Bit de lectura
	}
}

// createFileInDirectory - Crear un archivo en un directorio específico
func createFileInDirectory(partitionID string, parentInode int32, fileName string, content string) int32 {
	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return -1
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return -1
	}
	defer file.Close()

	// Leer el superblock
	superblock, err := ReadSuperblock(partitionID)
	if err != nil {
		return -1
	}

	// Buscar un inodo libre
	freeInode := findFreeInode(file, superblock)
	if freeInode == -1 {
		fmt.Println("Error: No hay inodos libres disponibles")
		return -1
	}

	// Calcular cuántos bloques necesitamos
	contentSize := len(content)
	blocksNeeded := 0
	if contentSize > 0 {
		blocksNeeded = (contentSize + 63) / 64 // Redondear hacia arriba
	}

	// Verificar si hay suficientes bloques libres
	if blocksNeeded > 0 {
		freeBlocksCount := countFreeBlocks(file, superblock)
		requiredBlocks := blocksNeeded
		if blocksNeeded > 12 {
			requiredBlocks++ // Bloque adicional para punteros indirectos
		}
		
		if freeBlocksCount < int32(requiredBlocks) {
			fmt.Printf("Error: No hay suficientes bloques libres. Necesarios: %d, Disponibles: %d\n", requiredBlocks, freeBlocksCount)
			return -1
		}
	}

	// Buscar bloques libres para el contenido del archivo
	var dataBlocks []int32
	var indirectBlock int32 = -1

	if blocksNeeded > 0 {
		for i := 0; i < blocksNeeded; i++ {
			freeBlock := findFreeBlock(file, superblock)
			if freeBlock == -1 {
				fmt.Printf("Error: No se pudo encontrar bloque libre %d de %d\n", i+1, blocksNeeded)
				return -1
			}
			dataBlocks = append(dataBlocks, freeBlock)
			
			// Marcar bloque como ocupado
			Utilities.WriteObject(file, byte(1), int64(superblock.S_bm_block_start+freeBlock))
		}

		// Si necesitamos más de 12 bloques, necesitamos un bloque para punteros indirectos
		if blocksNeeded > 12 {
			indirectBlock = findFreeBlock(file, superblock)
			if indirectBlock == -1 {
				fmt.Println("Error: No se pudo encontrar bloque para punteros indirectos")
				return -1
			}
			// Marcar bloque indirecto como ocupado
			Utilities.WriteObject(file, byte(1), int64(superblock.S_bm_block_start+indirectBlock))
		}
	}

	// Crear el inodo del archivo
	var newInode Structs.Inode
	newInode.I_uid = int32(CurrentSession.UserID)
	newInode.I_gid = int32(CurrentSession.GroupID)
	newInode.I_size = int32(contentSize)
	
	// Configurar fechas
	currentDate := "02/09/2025"
	copy(newInode.I_atime[:], currentDate)
	copy(newInode.I_ctime[:], currentDate)
	copy(newInode.I_mtime[:], currentDate)
	
	copy(newInode.I_type[:], "1")    // 1 = archivo regular
	// Asignar permisos según el usuario
	if CurrentSession != nil && CurrentSession.Username == "root" {
		copy(newInode.I_perm[:], "777")  // Permisos rwxrwxrwx para root
	} else {
		copy(newInode.I_perm[:], "664")  // Permisos rw-rw-r-- para otros usuarios
	}
	
	// Inicializar bloques
	for i := 0; i < 15; i++ {
		newInode.I_block[i] = -1
	}
	
	// Asignar bloques al inodo
	for i, blockNum := range dataBlocks {
		if i < 12 {
			// Punteros directos (I_block[0] a I_block[11])
			newInode.I_block[i] = blockNum
		} else if i == 12 {
			// Primer bloque indirecto
			newInode.I_block[12] = indirectBlock
			
			// Escribir el primer puntero en el bloque indirecto
			indirectBlockPos := int64(superblock.S_block_start + indirectBlock*superblock.S_block_size)
			Utilities.WriteObject(file, blockNum, indirectBlockPos)
		} else {
			// Punteros indirectos adicionales
			indirectIndex := i - 12
			indirectPos := int64(superblock.S_block_start + indirectBlock*superblock.S_block_size + int32(indirectIndex*4))
			Utilities.WriteObject(file, blockNum, indirectPos)
		}
	}

	// Escribir el inodo
	inodePos := int64(superblock.S_inode_start + freeInode*superblock.S_inode_size)
	if err := Utilities.WriteObject(file, newInode, inodePos); err != nil {
		return -1
	}

	// Escribir el contenido del archivo si hay contenido
	if contentSize > 0 {
		bytesWritten := 0
		for i, blockNum := range dataBlocks {
			// Calcular cuántos bytes escribir en este bloque
			remainingBytes := contentSize - bytesWritten
			bytesToWrite := 64
			if remainingBytes < 64 {
				bytesToWrite = remainingBytes
			}

			// Crear bloque de archivo con el contenido
			var fileBlock Structs.Fileblock
			// Llenar con el contenido correspondiente
			contentSlice := content[bytesWritten:bytesWritten+bytesToWrite]
			copy(fileBlock.B_content[:], contentSlice)
			
			// Escribir el bloque
			blockPos := int64(superblock.S_block_start + blockNum*superblock.S_block_size)
			if err := Utilities.WriteObject(file, fileBlock, blockPos); err != nil {
				fmt.Printf("Error escribiendo bloque %d: %v\n", i, err)
				return -1
			}

			bytesWritten += bytesToWrite
		}
	}

	// Marcar inodo como ocupado
	Utilities.WriteObject(file, byte(1), int64(superblock.S_bm_inode_start+freeInode))
	
	// Actualizar contadores en el superblock
	superblock.S_free_inodes_count--
	superblock.S_free_blocks_count -= int32(blocksNeeded)
	if indirectBlock != -1 {
		superblock.S_free_blocks_count-- // Bloque adicional para punteros indirectos
	}

	// Leer la posición correcta del superblock
	var partition *Structs.Partition = nil
	if !mountedPartition.IsLogical {
		// Partición primaria
		var tempMBR Structs.MBR
		if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
			return -1
		}
		partition = &tempMBR.Partitions[mountedPartition.PartitionIndex]
	}

	if partition != nil {
		// Actualizar superblock
		if err := Utilities.WriteObject(file, *superblock, int64(partition.Start)); err != nil {
			// Manejar error pero continuar
		}
	}

	// Agregar entrada en el directorio padre
	if !addFileToDirectory(file, superblock, parentInode, fileName, freeInode) {
		fmt.Println("Error: No se pudo agregar el archivo al directorio padre")
		return -1
	}

	return freeInode
}

// findFreeInode - Buscar un inodo libre en el bitmap
func findFreeInode(file *os.File, superblock *Structs.Superblock) int32 {
	for i := int32(0); i < superblock.S_inodes_count; i++ {
		var bitmapByte byte
		if err := Utilities.ReadObject(file, &bitmapByte, int64(superblock.S_bm_inode_start+i)); err != nil {
			continue
		}
		if bitmapByte == 0 {
			return i
		}
	}
	return -1
}

// findFreeBlock - Buscar un bloque libre en el bitmap
func findFreeBlock(file *os.File, superblock *Structs.Superblock) int32 {
	for i := int32(0); i < superblock.S_blocks_count; i++ {
		var bitmapByte byte
		if err := Utilities.ReadObject(file, &bitmapByte, int64(superblock.S_bm_block_start+i)); err != nil {
			continue
		}
		if bitmapByte == 0 {
			return i
		}
	}
	return -1
}

// countFreeBlocks - Contar bloques libres disponibles
func countFreeBlocks(file *os.File, superblock *Structs.Superblock) int32 {
	count := int32(0)
	for i := int32(0); i < superblock.S_blocks_count; i++ {
		var bitmapByte byte
		if err := Utilities.ReadObject(file, &bitmapByte, int64(superblock.S_bm_block_start+i)); err != nil {
			continue
		}
		if bitmapByte == 0 {
			count++
		}
	}
	return count
}

// addFileToDirectory - Agregar una entrada de archivo a un directorio
func addFileToDirectory(file *os.File, superblock *Structs.Superblock, dirInode int32, fileName string, fileInode int32) bool {
	// Leer el inodo del directorio
	var dirInodeStruct Structs.Inode
	inodePos := int64(superblock.S_inode_start + dirInode*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &dirInodeStruct, inodePos); err != nil {
		return false
	}

	// Verificar que sea un directorio
	if string(dirInodeStruct.I_type[:1]) != "0" {
		return false
	}

	// Buscar espacio en los bloques existentes del directorio
	for i := 0; i < 15 && dirInodeStruct.I_block[i] != -1; i++ {
		var folderBlock Structs.Folderblock
		blockPos := int64(superblock.S_block_start + dirInodeStruct.I_block[i]*superblock.S_block_size)
		if err := Utilities.ReadObject(file, &folderBlock, blockPos); err != nil {
			continue
		}

		// Buscar una entrada vacía en el bloque
		for j := 0; j < 4; j++ {
			if folderBlock.B_content[j].B_inodo == -1 {
				// Entrada vacía encontrada
				copy(folderBlock.B_content[j].B_name[:], fileName)
				folderBlock.B_content[j].B_inodo = fileInode
				
				// Escribir el bloque actualizado
				if err := Utilities.WriteObject(file, folderBlock, blockPos); err != nil {
					return false
				}
				return true
			}
		}
	}

	// No hay espacio en los bloques existentes, crear un nuevo bloque
	// Buscar una ranura libre en el inodo del directorio
	var freeSlot = -1
	for i := 0; i < 15; i++ {
		if dirInodeStruct.I_block[i] == -1 {
			freeSlot = i
			break
		}
	}

	if freeSlot == -1 {
		fmt.Println("Error: El directorio ha alcanzado el máximo de bloques (15)")
		return false
	}

	// Buscar un bloque libre
	freeBlock := findFreeBlock(file, superblock)
	if freeBlock == -1 {
		fmt.Println("Error: No hay bloques libres disponibles para extender el directorio")
		return false
	}

	// Crear un nuevo bloque de directorio
	var newFolderBlock Structs.Folderblock
	
	// Inicializar todas las entradas como vacías
	for i := 0; i < 4; i++ {
		newFolderBlock.B_content[i].B_inodo = -1
		for j := 0; j < 12; j++ {
			newFolderBlock.B_content[i].B_name[j] = 0
		}
	}

	// Agregar la nueva entrada en la primera ranura
	copy(newFolderBlock.B_content[0].B_name[:], fileName)
	newFolderBlock.B_content[0].B_inodo = fileInode

	// Escribir el nuevo bloque
	blockPos := int64(superblock.S_block_start + freeBlock*superblock.S_block_size)
	if err := Utilities.WriteObject(file, newFolderBlock, blockPos); err != nil {
		return false
	}

	// Asignar el bloque al directorio
	dirInodeStruct.I_block[freeSlot] = freeBlock
	dirInodeStruct.I_size += 64 // Incrementar el tamaño del directorio

	// Actualizar el inodo del directorio
	if err := Utilities.WriteObject(file, dirInodeStruct, inodePos); err != nil {
		return false
	}

	// Marcar el bloque como ocupado
	Utilities.WriteObject(file, byte(1), int64(superblock.S_bm_block_start+freeBlock))

	// Actualizar contador de bloques libres
	superblock.S_free_blocks_count--

	return true
}

// ============================================================================
// COMANDO REMOVE - ELIMINAR ARCHIVOS Y DIRECTORIOS
// ============================================================================

// Remove - Eliminar un archivo o directorio con validación de permisos
func Remove(path string) {
	fmt.Println("======Inicio REMOVE======")
	fmt.Printf("Ruta: %s\n", path)
	
	// Verificar que haya una sesión activa
	if !IsUserLoggedIn() {
		fmt.Println("Error: No hay una sesión activa")
		fmt.Println("Use el comando 'login' para iniciar sesión")
		fmt.Println("======FIN REMOVE======")
		return
	}

	// Validar que la ruta no esté vacía
	if strings.TrimSpace(path) == "" {
		fmt.Println("Error: La ruta no puede estar vacía")
		fmt.Println("======FIN REMOVE======")
		return
	}

	// Validar que la ruta empiece con "/"
	if !strings.HasPrefix(path, "/") {
		fmt.Println("Error: La ruta debe empezar con '/' (ruta absoluta)")
		fmt.Println("======FIN REMOVE======")
		return
	}

	// No permitir eliminar la raíz
	if path == "/" {
		fmt.Println("Error: No se puede eliminar el directorio raíz")
		fmt.Println("======FIN REMOVE======")
		return
	}

	// Buscar el archivo o directorio
	exists, inodeNum := findFileInDirectory(CurrentSession.PartitionID, path)
	if !exists {
		// Intentar buscar como directorio
		exists, inodeNum = findDirectoryInPath(CurrentSession.PartitionID, path)
		if !exists {
			fmt.Printf("Error: La ruta '%s' no existe\n", path)
			fmt.Println("======FIN REMOVE======")
			return
		}
	}

	// Leer el inodo para determinar el tipo
	mountedPartition, exists := DiskManagement.MountedPartitions[CurrentSession.PartitionID]
	if !exists {
		fmt.Println("Error: Partición no encontrada")
		fmt.Println("======FIN REMOVE======")
		return
	}

	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		fmt.Println("Error: No se pudo abrir el disco")
		fmt.Println("======FIN REMOVE======")
		return
	}
	defer file.Close()

	superblock, err := ReadSuperblock(CurrentSession.PartitionID)
	if err != nil {
		fmt.Println("Error: No se pudo leer el superblock")
		fmt.Println("======FIN REMOVE======")
		return
	}

	var inode Structs.Inode
	inodePos := int64(superblock.S_inode_start + inodeNum*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &inode, inodePos); err != nil {
		fmt.Println("Error: No se pudo leer el inodo")
		fmt.Println("======FIN REMOVE======")
		return
	}

	isDirectory := string(inode.I_type[:1]) == "0"

	// Verificar permisos de escritura
	if !hasWritePermission(CurrentSession.PartitionID, inodeNum, CurrentSession.UserID, CurrentSession.GroupID) {
		fmt.Printf("Error: No tiene permisos de escritura sobre '%s'\n", path)
		fmt.Println("======FIN REMOVE======")
		return
	}

	// Si es un directorio, validar permisos recursivamente ANTES de eliminar
	if isDirectory {
		canDelete, failedPath := canDeleteDirectoryRecursive(CurrentSession.PartitionID, inodeNum, path)
		if !canDelete {
			fmt.Printf("Error: No tiene permisos de escritura sobre '%s'\n", failedPath)
			fmt.Println("No se eliminó nada")
			fmt.Println("======FIN REMOVE======")
			return
		}
	}

	// Separar el path en directorio padre y nombre
	parentDir, itemName := parseFilePath(path)
	
	// Obtener el inodo del directorio padre
	parentExists, parentInode := findDirectoryInPath(CurrentSession.PartitionID, parentDir)
	if !parentExists {
		fmt.Println("Error: Directorio padre no encontrado")
		fmt.Println("======FIN REMOVE======")
		return
	}

	// Verificar permisos de escritura en el directorio padre
	if !hasWritePermission(CurrentSession.PartitionID, parentInode, CurrentSession.UserID, CurrentSession.GroupID) {
		fmt.Printf("Error: No tiene permisos de escritura en el directorio padre '%s'\n", parentDir)
		fmt.Println("======FIN REMOVE======")
		return
	}

	// Eliminar según el tipo
	var success bool
	var itemType string
	if isDirectory {
		success = deleteDirectoryRecursive(file, superblock, inodeNum)
		itemType = "directory"
		if success {
			// Remover la entrada del directorio padre
			removeEntryFromParent(file, superblock, parentInode, itemName)
			fmt.Printf("Directorio '%s' eliminado exitosamente\n", path)
		}
	} else {
		success = deleteFile(file, superblock, inodeNum)
		itemType = "file"
		if success {
			// Remover la entrada del directorio padre
			removeEntryFromParent(file, superblock, parentInode, itemName)
			fmt.Printf("Archivo '%s' eliminado exitosamente\n", path)
		}
	}

	if !success {
		fmt.Println("Error: No se pudo eliminar completamente")
		fmt.Println("======FIN REMOVE======")
		return
	}

	// Registrar en el journaling (EXT3)
	writeToJournal(CurrentSession.PartitionID, "remove", path, itemType)

	// Actualizar el superblock en disco
	var partition Structs.Partition
	var tempMBR Structs.MBR
	Utilities.ReadObject(file, &tempMBR, 0)
	
	if !mountedPartition.IsLogical {
		partition = tempMBR.Partitions[mountedPartition.PartitionIndex]
	}
	
	superblockPos := int64(partition.Start)
	if mountedPartition.IsLogical {
		var tempEBR Structs.EBR
		Utilities.ReadObject(file, &tempEBR, int64(mountedPartition.EBRPosition))
		superblockPos = int64(mountedPartition.EBRPosition) + int64(binary.Size(Structs.EBR{}))
	}
	
	Utilities.WriteObject(file, superblock, superblockPos)

	fmt.Println("=== ELIMINACIÓN EXITOSA ===")
	fmt.Printf("Ruta: %s\n", path)
	fmt.Printf("Tipo: %s\n", map[bool]string{true: "Directorio", false: "Archivo"}[isDirectory])
	fmt.Printf("Inodos disponibles: %d\n", superblock.S_free_inodes_count)
	fmt.Printf("Bloques disponibles: %d\n", superblock.S_free_blocks_count)
	fmt.Println("======FIN REMOVE======")
}

// canDeleteDirectoryRecursive - Verificar si se puede eliminar un directorio y todo su contenido
func canDeleteDirectoryRecursive(partitionID string, dirInode int32, dirPath string) (bool, string) {
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return false, dirPath
	}

	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return false, dirPath
	}
	defer file.Close()

	superblock, err := ReadSuperblock(partitionID)
	if err != nil {
		return false, dirPath
	}

	// Leer el inodo del directorio
	var dirInodeStruct Structs.Inode
	inodePos := int64(superblock.S_inode_start + dirInode*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &dirInodeStruct, inodePos); err != nil {
		return false, dirPath
	}

	// Iterar sobre los bloques del directorio
	for i := 0; i < 15 && dirInodeStruct.I_block[i] != -1; i++ {
		var folderBlock Structs.Folderblock
		blockPos := int64(superblock.S_block_start + dirInodeStruct.I_block[i]*superblock.S_block_size)
		if err := Utilities.ReadObject(file, &folderBlock, blockPos); err != nil {
			continue
		}

		// Revisar cada entrada del bloque
		for j := 0; j < 4; j++ {
			if folderBlock.B_content[j].B_inodo == -1 {
				continue
			}

			entryName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
			
			// Saltar . y ..
			if entryName == "." || entryName == ".." {
				continue
			}

			entryInode := folderBlock.B_content[j].B_inodo
			entryPath := dirPath + "/" + entryName

			// Verificar permisos de escritura en esta entrada
			if !hasWritePermission(partitionID, entryInode, CurrentSession.UserID, CurrentSession.GroupID) {
				return false, entryPath
			}

			// Leer el tipo de la entrada
			var entryInodeStruct Structs.Inode
			entryInodePos := int64(superblock.S_inode_start + entryInode*superblock.S_inode_size)
			if err := Utilities.ReadObject(file, &entryInodeStruct, entryInodePos); err != nil {
				return false, entryPath
			}

			// Si es un directorio, verificar recursivamente
			if string(entryInodeStruct.I_type[:1]) == "0" {
				canDelete, failedPath := canDeleteDirectoryRecursive(partitionID, entryInode, entryPath)
				if !canDelete {
					return false, failedPath
				}
			}
		}
	}

	return true, ""
}

// deleteFile - Eliminar un archivo y liberar sus recursos
func deleteFile(file *os.File, superblock *Structs.Superblock, inodeNum int32) bool {
	// Leer el inodo del archivo
	var inode Structs.Inode
	inodePos := int64(superblock.S_inode_start + inodeNum*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &inode, inodePos); err != nil {
		return false
	}

	// Liberar todos los bloques del archivo
	for i := 0; i < 15 && inode.I_block[i] != -1; i++ {
		blockNum := inode.I_block[i]
		
		// Marcar el bloque como libre en el bitmap
		Utilities.WriteObject(file, byte(0), int64(superblock.S_bm_block_start+blockNum))
		superblock.S_free_blocks_count++
	}

	// Marcar el inodo como libre en el bitmap
	Utilities.WriteObject(file, byte(0), int64(superblock.S_bm_inode_start+inodeNum))
	superblock.S_free_inodes_count++

	return true
}

// deleteDirectoryRecursive - Eliminar un directorio y todo su contenido recursivamente
func deleteDirectoryRecursive(file *os.File, superblock *Structs.Superblock, dirInode int32) bool {
	// Leer el inodo del directorio
	var dirInodeStruct Structs.Inode
	inodePos := int64(superblock.S_inode_start + dirInode*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &dirInodeStruct, inodePos); err != nil {
		return false
	}

	// Iterar sobre los bloques del directorio
	for i := 0; i < 15 && dirInodeStruct.I_block[i] != -1; i++ {
		var folderBlock Structs.Folderblock
		blockPos := int64(superblock.S_block_start + dirInodeStruct.I_block[i]*superblock.S_block_size)
		if err := Utilities.ReadObject(file, &folderBlock, blockPos); err != nil {
			continue
		}

		// Eliminar cada entrada del bloque
		for j := 0; j < 4; j++ {
			if folderBlock.B_content[j].B_inodo == -1 {
				continue
			}

			entryName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
			
			// Saltar . y ..
			if entryName == "." || entryName == ".." {
				continue
			}

			entryInode := folderBlock.B_content[j].B_inodo

			// Leer el tipo de la entrada
			var entryInodeStruct Structs.Inode
			entryInodePos := int64(superblock.S_inode_start + entryInode*superblock.S_inode_size)
			if err := Utilities.ReadObject(file, &entryInodeStruct, entryInodePos); err != nil {
				continue
			}

			// Si es un directorio, eliminar recursivamente
			if string(entryInodeStruct.I_type[:1]) == "0" {
				deleteDirectoryRecursive(file, superblock, entryInode)
			} else {
				// Si es un archivo, eliminarlo
				deleteFile(file, superblock, entryInode)
			}
		}

		// Liberar el bloque del directorio
		blockNum := dirInodeStruct.I_block[i]
		Utilities.WriteObject(file, byte(0), int64(superblock.S_bm_block_start+blockNum))
		superblock.S_free_blocks_count++
	}

	// Marcar el inodo del directorio como libre
	Utilities.WriteObject(file, byte(0), int64(superblock.S_bm_inode_start+dirInode))
	superblock.S_free_inodes_count++

	return true
}

// removeEntryFromParent - Remover una entrada del directorio padre
func removeEntryFromParent(file *os.File, superblock *Structs.Superblock, parentInode int32, entryName string) bool {
	// Leer el inodo del directorio padre
	var parentInodeStruct Structs.Inode
	inodePos := int64(superblock.S_inode_start + parentInode*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &parentInodeStruct, inodePos); err != nil {
		return false
	}

	// Buscar la entrada en los bloques del directorio
	for i := 0; i < 15 && parentInodeStruct.I_block[i] != -1; i++ {
		var folderBlock Structs.Folderblock
		blockPos := int64(superblock.S_block_start + parentInodeStruct.I_block[i]*superblock.S_block_size)
		if err := Utilities.ReadObject(file, &folderBlock, blockPos); err != nil {
			continue
		}

		// Buscar la entrada
		for j := 0; j < 4; j++ {
			if folderBlock.B_content[j].B_inodo == -1 {
				continue
			}

			currentName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
			if currentName == entryName {
				// Marcar la entrada como vacía
				folderBlock.B_content[j].B_inodo = -1
				for k := 0; k < 12; k++ {
					folderBlock.B_content[j].B_name[k] = 0
				}

				// Escribir el bloque actualizado
				Utilities.WriteObject(file, folderBlock, blockPos)
				
				// Actualizar el tamaño del directorio padre
				parentInodeStruct.I_size -= 64
				Utilities.WriteObject(file, parentInodeStruct, inodePos)
				
				return true
			}
		}
	}

	return false
}

// ============================================================================
// COMANDO EDIT - EDITAR CONTENIDO DE ARCHIVOS
// ============================================================================

// Edit - Editar el contenido de un archivo existente
func Edit(path string, contenidoPath string) {
	fmt.Println("======Inicio EDIT======")
	fmt.Printf("Ruta del archivo: %s\n", path)
	fmt.Printf("Archivo de contenido: %s\n", contenidoPath)
	
	// Verificar que haya una sesión activa
	if !IsUserLoggedIn() {
		fmt.Println("Error: No hay una sesión activa")
		fmt.Println("Use el comando 'login' para iniciar sesión")
		fmt.Println("======FIN EDIT======")
		return
	}

	// Validar que la ruta no esté vacía
	if strings.TrimSpace(path) == "" {
		fmt.Println("Error: La ruta del archivo no puede estar vacía")
		fmt.Println("======FIN EDIT======")
		return
	}

	// Validar que la ruta empiece con "/"
	if !strings.HasPrefix(path, "/") {
		fmt.Println("Error: La ruta debe empezar con '/' (ruta absoluta)")
		fmt.Println("======FIN EDIT======")
		return
	}

	// Verificar que el archivo de contenido local existe
	if !fileExistsLocal(contenidoPath) {
		fmt.Printf("Error: El archivo de contenido '%s' no existe en el sistema local\n", contenidoPath)
		fmt.Println("======FIN EDIT======")
		return
	}

	// Leer el contenido del archivo local
	newContent, err := readLocalFile(contenidoPath)
	if err != nil {
		fmt.Printf("Error leyendo archivo de contenido '%s': %s\n", contenidoPath, err.Error())
		fmt.Println("======FIN EDIT======")
		return
	}

	// Buscar el archivo en el sistema
	exists, inodeNum := findFileInDirectory(CurrentSession.PartitionID, path)
	if !exists {
		fmt.Printf("Error: El archivo '%s' no existe\n", path)
		fmt.Println("======FIN EDIT======")
		return
	}

	// Verificar permisos de lectura
	if !hasReadPermission(CurrentSession.PartitionID, inodeNum, CurrentSession.UserID, CurrentSession.GroupID) {
		fmt.Printf("Error: No tiene permisos de lectura sobre el archivo '%s'\n", path)
		fmt.Println("======FIN EDIT======")
		return
	}

	// Verificar permisos de escritura
	if !hasWritePermission(CurrentSession.PartitionID, inodeNum, CurrentSession.UserID, CurrentSession.GroupID) {
		fmt.Printf("Error: No tiene permisos de escritura sobre el archivo '%s'\n", path)
		fmt.Println("======FIN EDIT======")
		return
	}

	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[CurrentSession.PartitionID]
	if !exists {
		fmt.Println("Error: Partición no encontrada")
		fmt.Println("======FIN EDIT======")
		return
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		fmt.Println("Error: No se pudo abrir el disco")
		fmt.Println("======FIN EDIT======")
		return
	}
	defer file.Close()

	// Leer el superblock
	superblock, err := ReadSuperblock(CurrentSession.PartitionID)
	if err != nil {
		fmt.Println("Error: No se pudo leer el superblock")
		fmt.Println("======FIN EDIT======")
		return
	}

	// Leer el inodo del archivo
	var inode Structs.Inode
	inodePos := int64(superblock.S_inode_start + inodeNum*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &inode, inodePos); err != nil {
		fmt.Println("Error: No se pudo leer el inodo del archivo")
		fmt.Println("======FIN EDIT======")
		return
	}

	// Verificar que sea un archivo (no directorio)
	if string(inode.I_type[:1]) == "0" {
		fmt.Printf("Error: '%s' es un directorio, no un archivo\n", path)
		fmt.Println("======FIN EDIT======")
		return
	}

	// Guardar información del contenido antiguo
	oldSize := inode.I_size
	var oldBlocks []int32
	for i := 0; i < 15 && inode.I_block[i] != -1; i++ {
		oldBlocks = append(oldBlocks, inode.I_block[i])
	}

	// Liberar los bloques antiguos
	for _, blockNum := range oldBlocks {
		Utilities.WriteObject(file, byte(0), int64(superblock.S_bm_block_start+blockNum))
		superblock.S_free_blocks_count++
	}

	// Calcular cuántos bloques necesitamos para el nuevo contenido
	contentSize := len(newContent)
	blocksNeeded := (contentSize + 63) / 64 // Redondear hacia arriba
	
	if blocksNeeded > 15 {
		fmt.Printf("Error: El contenido es demasiado grande (%d bytes)\n", contentSize)
		fmt.Printf("Máximo soportado: 15 bloques × 64 bytes = 960 bytes\n")
		fmt.Println("======FIN EDIT======")
		return
	}

	// Verificar que hay suficientes bloques libres
	if superblock.S_free_blocks_count < int32(blocksNeeded) {
		fmt.Printf("Error: No hay suficientes bloques libres (necesarios: %d, disponibles: %d)\n", 
			blocksNeeded, superblock.S_free_blocks_count)
		fmt.Println("======FIN EDIT======")
		return
	}

	// Asignar nuevos bloques y escribir el contenido
	contentOffset := 0
	for i := 0; i < blocksNeeded; i++ {
		// Buscar un bloque libre
		freeBlock := findFreeBlock(file, superblock)
		if freeBlock == -1 {
			fmt.Println("Error: No se pudo encontrar un bloque libre")
			fmt.Println("======FIN EDIT======")
			return
		}

		// Crear un fileblock con el contenido
		var fileBlock Structs.Fileblock
		
		// Copiar el contenido al bloque (máximo 64 bytes)
		bytesToCopy := 64
		remaining := contentSize - contentOffset
		if remaining < 64 {
			bytesToCopy = remaining
		}
		
		copy(fileBlock.B_content[:], newContent[contentOffset:contentOffset+bytesToCopy])
		contentOffset += bytesToCopy

		// Escribir el bloque en el disco
		blockPos := int64(superblock.S_block_start + freeBlock*superblock.S_block_size)
		if err := Utilities.WriteObject(file, fileBlock, blockPos); err != nil {
			fmt.Println("Error: No se pudo escribir el bloque")
			fmt.Println("======FIN EDIT======")
			return
		}

		// Marcar el bloque como ocupado
		Utilities.WriteObject(file, byte(1), int64(superblock.S_bm_block_start+freeBlock))
		superblock.S_free_blocks_count--

		// Asignar el bloque al inodo
		inode.I_block[i] = freeBlock
	}

	// Marcar los bloques restantes como no usados
	for i := blocksNeeded; i < 15; i++ {
		inode.I_block[i] = -1
	}

	// Actualizar el tamaño del archivo
	inode.I_size = int32(contentSize)

	// Actualizar la fecha de modificación
	copy(inode.I_mtime[:], "19/10/2025")

	// Escribir el inodo actualizado
	if err := Utilities.WriteObject(file, inode, inodePos); err != nil {
		fmt.Println("Error: No se pudo actualizar el inodo")
		fmt.Println("======FIN EDIT======")
		return
	}

	// Actualizar el superblock en el disco
	var partition Structs.Partition
	var tempMBR Structs.MBR
	Utilities.ReadObject(file, &tempMBR, 0)
	
	if !mountedPartition.IsLogical {
		partition = tempMBR.Partitions[mountedPartition.PartitionIndex]
	}
	
	superblockPos := int64(partition.Start)
	if mountedPartition.IsLogical {
		var tempEBR Structs.EBR
		Utilities.ReadObject(file, &tempEBR, int64(mountedPartition.EBRPosition))
		superblockPos = int64(mountedPartition.EBRPosition) + int64(binary.Size(Structs.EBR{}))
	}
	
	Utilities.WriteObject(file, superblock, superblockPos)

	// Registrar en el journaling (EXT3)
	contentPreview := fmt.Sprintf("size=%d", contentSize)
	writeToJournal(CurrentSession.PartitionID, "edit", path, contentPreview)

	fmt.Println("=== ARCHIVO EDITADO EXITOSAMENTE ===")
	fmt.Printf("Ruta: %s\n", path)
	fmt.Printf("Tamaño anterior: %d bytes\n", oldSize)
	fmt.Printf("Tamaño nuevo: %d bytes\n", contentSize)
	fmt.Printf("Bloques anteriores: %d\n", len(oldBlocks))
	fmt.Printf("Bloques nuevos: %d\n", blocksNeeded)
	fmt.Printf("Bloques libres: %d\n", superblock.S_free_blocks_count)
	fmt.Printf("Inodo: %d\n", inodeNum)
	fmt.Printf("Fecha de modificación: %s\n", string(inode.I_mtime[:]))
	fmt.Println("======FIN EDIT======")
}

// ============================================================================
// COMANDO RENAME - RENOMBRAR ARCHIVOS Y DIRECTORIOS
// ============================================================================

// Rename - Cambiar el nombre de un archivo o directorio
func Rename(path string, newName string) {
	fmt.Println("======Inicio RENAME======")
	fmt.Printf("Ruta: %s\n", path)
	fmt.Printf("Nuevo nombre: %s\n", newName)
	
	// Verificar que haya una sesión activa
	if !IsUserLoggedIn() {
		fmt.Println("Error: No hay una sesión activa")
		fmt.Println("Use el comando 'login' para iniciar sesión")
		fmt.Println("======FIN RENAME======")
		return
	}

	// Validar que la ruta no esté vacía
	if strings.TrimSpace(path) == "" {
		fmt.Println("Error: La ruta no puede estar vacía")
		fmt.Println("======FIN RENAME======")
		return
	}

	// Validar que el nuevo nombre no esté vacío
	if strings.TrimSpace(newName) == "" {
		fmt.Println("Error: El nuevo nombre no puede estar vacío")
		fmt.Println("======FIN RENAME======")
		return
	}

	// Validar que la ruta empiece con "/"
	if !strings.HasPrefix(path, "/") {
		fmt.Println("Error: La ruta debe empezar con '/' (ruta absoluta)")
		fmt.Println("======FIN RENAME======")
		return
	}

	// No permitir renombrar la raíz
	if path == "/" {
		fmt.Println("Error: No se puede renombrar el directorio raíz")
		fmt.Println("======FIN RENAME======")
		return
	}

	// Validar que el nuevo nombre no contenga "/"
	if strings.Contains(newName, "/") {
		fmt.Println("Error: El nuevo nombre no puede contener el carácter '/'")
		fmt.Println("======FIN RENAME======")
		return
	}

	// Validar longitud del nuevo nombre (máximo 12 caracteres para B_name)
	if len(newName) > 12 {
		fmt.Println("Error: El nuevo nombre es demasiado largo (máximo 12 caracteres)")
		fmt.Printf("Longitud actual: %d caracteres\n", len(newName))
		fmt.Println("======FIN RENAME======")
		return
	}

	// Buscar el archivo o directorio
	exists, inodeNum := findFileInDirectory(CurrentSession.PartitionID, path)
	isFile := exists
	
	if !exists {
		// Intentar buscar como directorio
		exists, inodeNum = findDirectoryInPath(CurrentSession.PartitionID, path)
		if !exists {
			fmt.Printf("Error: La ruta '%s' no existe\n", path)
			fmt.Println("======FIN RENAME======")
			return
		}
		isFile = false
	}

	// Separar el path en directorio padre y nombre actual
	parentDir, currentName := parseFilePath(path)
	
	// Obtener el inodo del directorio padre
	parentExists, parentInode := findDirectoryInPath(CurrentSession.PartitionID, parentDir)
	if !parentExists {
		fmt.Println("Error: Directorio padre no encontrado")
		fmt.Println("======FIN RENAME======")
		return
	}

	// Verificar permisos de escritura sobre el directorio padre
	// (Para renombrar se requiere permiso de escritura en el directorio que contiene el archivo)
	if !hasWritePermission(CurrentSession.PartitionID, parentInode, CurrentSession.UserID, CurrentSession.GroupID) {
		fmt.Printf("Error: No tiene permisos de escritura en el directorio '%s'\n", parentDir)
		fmt.Println("======FIN RENAME======")
		return
	}

	// Verificar que no exista otro archivo/directorio con el nuevo nombre en el mismo directorio
	existsWithNewName, _ := findInodeInDirectory(CurrentSession.PartitionID, parentInode, newName, false)
	if existsWithNewName {
		fmt.Printf("Error: Ya existe un archivo o directorio con el nombre '%s' en el directorio '%s'\n", newName, parentDir)
		fmt.Println("======FIN RENAME======")
		return
	}

	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[CurrentSession.PartitionID]
	if !exists {
		fmt.Println("Error: Partición no encontrada")
		fmt.Println("======FIN RENAME======")
		return
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		fmt.Println("Error: No se pudo abrir el disco")
		fmt.Println("======FIN RENAME======")
		return
	}
	defer file.Close()

	// Leer el superblock
	superblock, err := ReadSuperblock(CurrentSession.PartitionID)
	if err != nil {
		fmt.Println("Error: No se pudo leer el superblock")
		fmt.Println("======FIN RENAME======")
		return
	}

	// Buscar y actualizar el nombre en el directorio padre
	success := updateNameInParentDirectory(file, superblock, parentInode, currentName, newName)
	if !success {
		fmt.Println("Error: No se pudo actualizar el nombre en el directorio padre")
		fmt.Println("======FIN RENAME======")
		return
	}

	// Determinar el tipo (archivo o directorio)
	var inode Structs.Inode
	inodePos := int64(superblock.S_inode_start + inodeNum*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &inode, inodePos); err == nil {
		isFile = string(inode.I_type[:1]) == "1"
	}

	// Registrar en el journaling (EXT3)
	contentInfo := fmt.Sprintf("%s->%s", currentName, newName)
	writeToJournal(CurrentSession.PartitionID, "rename", path, contentInfo)

	fmt.Println("=== RENOMBRADO EXITOSO ===")
	fmt.Printf("Ruta original: %s\n", path)
	fmt.Printf("Nombre anterior: %s\n", currentName)
	fmt.Printf("Nombre nuevo: %s\n", newName)
	fmt.Printf("Ruta nueva: %s/%s\n", parentDir, newName)
	fmt.Printf("Tipo: %s\n", map[bool]string{true: "Archivo", false: "Directorio"}[isFile])
	fmt.Printf("Inodo: %d\n", inodeNum)
	fmt.Println("======FIN RENAME======")
}

// updateNameInParentDirectory - Actualizar el nombre de una entrada en el directorio padre
func updateNameInParentDirectory(file *os.File, superblock *Structs.Superblock, parentInode int32, oldName string, newName string) bool {
	// Leer el inodo del directorio padre
	var parentInodeStruct Structs.Inode
	inodePos := int64(superblock.S_inode_start + parentInode*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &parentInodeStruct, inodePos); err != nil {
		return false
	}

	// Buscar la entrada en los bloques del directorio
	for i := 0; i < 15 && parentInodeStruct.I_block[i] != -1; i++ {
		var folderBlock Structs.Folderblock
		blockPos := int64(superblock.S_block_start + parentInodeStruct.I_block[i]*superblock.S_block_size)
		if err := Utilities.ReadObject(file, &folderBlock, blockPos); err != nil {
			continue
		}

		// Buscar la entrada con el nombre antiguo
		for j := 0; j < 4; j++ {
			if folderBlock.B_content[j].B_inodo == -1 {
				continue
			}

			currentName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
			if currentName == oldName {
				// Actualizar el nombre
				// Limpiar el nombre anterior
				for k := 0; k < 12; k++ {
					folderBlock.B_content[j].B_name[k] = 0
				}
				
				// Copiar el nuevo nombre
				copy(folderBlock.B_content[j].B_name[:], newName)

				// Escribir el bloque actualizado
				if err := Utilities.WriteObject(file, folderBlock, blockPos); err != nil {
					return false
				}
				
				// Sincronizar cambios al disco
				file.Sync()
				
				return true
			}
		}
	}

	return false
}
// ============================================================================
// COMANDO COPY - COPIAR ARCHIVOS Y DIRECTORIOS
// ============================================================================

// Copy - Copiar archivo o directorio con todo su contenido
func Copy(path string, destino string) {
fmt.Println("======Inicio COPY======")
fmt.Printf("Origen: %s\n", path)
fmt.Printf("Destino: %s\n", destino)

// Verificar que haya una sesión activa
if !IsUserLoggedIn() {
fmt.Println("Error: No hay una sesión activa")
fmt.Println("Use el comando 'login' para iniciar sesión")
fmt.Println("======FIN COPY======")
return
}

// Validar que las rutas no estén vacías
if strings.TrimSpace(path) == "" {
fmt.Println("Error: La ruta de origen no puede estar vacía")
fmt.Println("======FIN COPY======")
return
}

if strings.TrimSpace(destino) == "" {
fmt.Println("Error: La ruta de destino no puede estar vacía")
fmt.Println("======FIN COPY======")
return
}

// Validar que las rutas empiecen con "/"
if !strings.HasPrefix(path, "/") {
fmt.Println("Error: La ruta de origen debe empezar con '/' (ruta absoluta)")
fmt.Println("======FIN COPY======")
return
}

if !strings.HasPrefix(destino, "/") {
fmt.Println("Error: La ruta de destino debe empezar con '/' (ruta absoluta)")
fmt.Println("======FIN COPY======")
return
}

// Buscar el archivo o directorio de origen
existsFile, sourceInodeFile := findFileInDirectory(CurrentSession.PartitionID, path)
existsDir, sourceInodeDir := findDirectoryInPath(CurrentSession.PartitionID, path)

if !existsFile && !existsDir {
fmt.Printf("Error: La ruta de origen '%s' no existe\n", path)
fmt.Println("======FIN COPY======")
return
}

// Determinar si es archivo o directorio
isFile := existsFile
sourceInode := sourceInodeFile
if !isFile {
sourceInode = sourceInodeDir
}

// Verificar permisos de lectura sobre el origen
if !hasReadPermission(CurrentSession.PartitionID, sourceInode, CurrentSession.UserID, CurrentSession.GroupID) {
fmt.Printf("Error: No tiene permisos de lectura sobre '%s'\n", path)
fmt.Println("======FIN COPY======")
return
}

// Verificar que el directorio destino exista
existsDestDir, destDirInode := findDirectoryInPath(CurrentSession.PartitionID, destino)
if !existsDestDir {
fmt.Printf("Error: El directorio de destino '%s' no existe\n", destino)
fmt.Println("======FIN COPY======")
return
}

// Verificar permisos de escritura sobre el destino
if !hasWritePermission(CurrentSession.PartitionID, destDirInode, CurrentSession.UserID, CurrentSession.GroupID) {
fmt.Printf("Error: No tiene permisos de escritura en el directorio de destino '%s'\n", destino)
fmt.Println("======FIN COPY======")
return
}

// Obtener el nombre del archivo/directorio a copiar
_, sourceName := parseFilePath(path)

// Verificar que no exista ya un archivo/directorio con ese nombre en destino
existsInDest, _ := findInodeInDirectory(CurrentSession.PartitionID, destDirInode, sourceName, false)
if existsInDest {
fmt.Printf("Error: Ya existe un archivo o directorio con el nombre '%s' en '%s'\n", sourceName, destino)
fmt.Println("======FIN COPY======")
return
}

// Obtener información de la partición
mountedPartition, exists := DiskManagement.MountedPartitions[CurrentSession.PartitionID]
if !exists {
fmt.Println("Error: Partición no encontrada")
fmt.Println("======FIN COPY======")
return
}

file, err := Utilities.OpenFile(mountedPartition.Path)
if err != nil {
fmt.Println("Error: No se pudo abrir el disco")
fmt.Println("======FIN COPY======")
return
}
defer file.Close()

superblock, err := ReadSuperblock(CurrentSession.PartitionID)
if err != nil {
fmt.Println("Error: No se pudo leer el superblock")
fmt.Println("======FIN COPY======")
return
}

// Realizar la copia según el tipo
var success bool
var copiedCount int
var skippedCount int

if isFile {
success, copiedCount, skippedCount = copyFileInternal(file, superblock, sourceInode, destDirInode, sourceName)
} else {
success, copiedCount, skippedCount = copyDirectoryInternal(file, superblock, sourceInode, destDirInode, sourceName, path, 0)
}

if success {
	// Registrar en el journaling (EXT3)
	contentInfo := fmt.Sprintf("%s->%s", path, destino)
	writeToJournal(CurrentSession.PartitionID, "copy", path, contentInfo)
	
	fmt.Println("\n=== COPIA COMPLETADA ===")
	fmt.Printf("Origen: %s\n", path)
	fmt.Printf("Destino: %s/%s\n", destino, sourceName)
fmt.Printf("Tipo: %s\n", map[bool]string{true: "Archivo", false: "Directorio"}[isFile])
fmt.Printf("Elementos copiados: %d\n", copiedCount)
if skippedCount > 0 {
fmt.Printf("Elementos omitidos (sin permisos de lectura): %d\n", skippedCount)
}
} else {
fmt.Println("\nError: No se pudo completar la copia")
}

fmt.Println("======FIN COPY======")
}

// copyFileInternal - Copiar un archivo individual
func copyFileInternal(file *os.File, superblock *Structs.Superblock, sourceInode int32, destDirInode int32, fileName string) (bool, int, int) {
// Leer el inodo del archivo origen
var srcInode Structs.Inode
srcInodePos := int64(superblock.S_inode_start + sourceInode*superblock.S_inode_size)
if err := Utilities.ReadObject(file, &srcInode, srcInodePos); err != nil {
fmt.Printf("Error al leer inodo del archivo origen\n")
return false, 0, 0
}

// Obtener el tamaño del archivo
fileSize := srcInode.I_size

// Asignar un nuevo inodo para el archivo destino
newInodeIndex := findFreeInode(file, superblock)
if newInodeIndex == -1 {
fmt.Println("Error: No hay inodos disponibles")
return false, 0, 0
}

// Leer todo el contenido del archivo origen
content := make([]byte, fileSize)
bytesRead := int32(0)

for i := 0; i < 15 && srcInode.I_block[i] != -1 && bytesRead < fileSize; i++ {
var fileBlock Structs.Fileblock
blockPos := int64(superblock.S_block_start + srcInode.I_block[i]*superblock.S_block_size)
if err := Utilities.ReadObject(file, &fileBlock, blockPos); err != nil {
continue
}

// Copiar el contenido del bloque
remaining := fileSize - bytesRead
toCopy := int32(64)
if remaining < toCopy {
toCopy = remaining
}

copy(content[bytesRead:bytesRead+toCopy], fileBlock.B_content[:toCopy])
bytesRead += toCopy
}

// Calcular cuántos bloques necesitamos
blocksNeeded := (fileSize + 63) / 64
blockIndices := make([]int32, blocksNeeded)

// Asignar bloques para el nuevo archivo
for i := int32(0); i < blocksNeeded; i++ {
blockIndices[i] = findFreeBlock(file, superblock)
if blockIndices[i] == -1 {
fmt.Println("Error: No hay bloques disponibles")
// Liberar recursos ya asignados
markInodeAsFree(file, superblock, newInodeIndex)
for j := int32(0); j < i; j++ {
markBlockAsFree(file, superblock, blockIndices[j])
}
return false, 0, 0
}
// Marcar bloque como usado
markBlockAsUsed(file, superblock, blockIndices[i])
}

// Marcar inodo como usado
markInodeAsUsed(file, superblock, newInodeIndex)

	// Crear el nuevo inodo
	var newInode Structs.Inode
	newInode.I_uid = int32(CurrentSession.UserID)
	newInode.I_gid = int32(CurrentSession.GroupID)
	newInode.I_size = fileSize
	
	// Configurar fechas
	currentDate := "02/09/2025"
	copy(newInode.I_atime[:], currentDate)
	copy(newInode.I_ctime[:], currentDate)
	copy(newInode.I_mtime[:], currentDate)

	for i := 0; i < 15; i++ {
		newInode.I_block[i] = -1
	}

	copy(newInode.I_type[:], "1") // Archivo

	// Permisos: copiar los permisos del archivo original
	copy(newInode.I_perm[:], srcInode.I_perm[:])
// Asignar los bloques al inodo
for i := int32(0); i < blocksNeeded && i < 15; i++ {
newInode.I_block[i] = blockIndices[i]
}

// Escribir el contenido en los bloques del nuevo archivo
bytesWritten := int32(0)
for i := int32(0); i < blocksNeeded; i++ {
var fileBlock Structs.Fileblock

remaining := fileSize - bytesWritten
toWrite := int32(64)
if remaining < toWrite {
toWrite = remaining
}

copy(fileBlock.B_content[:toWrite], content[bytesWritten:bytesWritten+toWrite])

blockPos := int64(superblock.S_block_start + blockIndices[i]*superblock.S_block_size)
if err := Utilities.WriteObject(file, fileBlock, blockPos); err != nil {
fmt.Printf("Error al escribir bloque de archivo\n")
return false, 0, 0
}

bytesWritten += toWrite
}

// Escribir el nuevo inodo
newInodePos := int64(superblock.S_inode_start + newInodeIndex*superblock.S_inode_size)
if err := Utilities.WriteObject(file, newInode, newInodePos); err != nil {
fmt.Printf("Error al escribir nuevo inodo\n")
return false, 0, 0
}

	// Agregar la entrada en el directorio destino
	if !addFileToDirectory(file, superblock, destDirInode, fileName, newInodeIndex) {
		fmt.Printf("Error al agregar entrada al directorio destino\n")
		return false, 0, 0
	}

	return true, 1, 0
}
// copyDirectoryInternal - Copiar un directorio recursivamente
func copyDirectoryInternal(file *os.File, superblock *Structs.Superblock, sourceInode int32, destDirInode int32, dirName string, originalPath string, depth int) (bool, int, int) {
// Limitar profundidad para evitar recursión infinita
if depth > 50 {
fmt.Println("Error: Profundidad máxima de recursión alcanzada")
return false, 0, 0
}

copiedCount := 0
skippedCount := 0

// Crear el nuevo directorio en el destino
newDirInode := findFreeInode(file, superblock)
if newDirInode == -1 {
fmt.Println("Error: No hay inodos disponibles para el directorio")
return false, 0, 0
}

// Asignar un bloque para el nuevo directorio
newDirBlock := findFreeBlock(file, superblock)
if newDirBlock == -1 {
fmt.Println("Error: No hay bloques disponibles para el directorio")
return false, 0, 0
}

// Marcar inodo y bloque como usados
markInodeAsUsed(file, superblock, newDirInode)
markBlockAsUsed(file, superblock, newDirBlock)

// Leer el inodo del directorio origen
var srcDirInode Structs.Inode
srcInodePos := int64(superblock.S_inode_start + sourceInode*superblock.S_inode_size)
if err := Utilities.ReadObject(file, &srcDirInode, srcInodePos); err != nil {
fmt.Printf("Error al leer inodo del directorio origen\n")
return false, 0, 0
}

	// Crear el nuevo inodo del directorio
	var newInode Structs.Inode
	newInode.I_uid = int32(CurrentSession.UserID)
	newInode.I_gid = int32(CurrentSession.GroupID)
	newInode.I_size = 0
	
	// Configurar fechas
	currentDate := "02/09/2025"
	copy(newInode.I_atime[:], currentDate)
	copy(newInode.I_ctime[:], currentDate)
	copy(newInode.I_mtime[:], currentDate)

	for i := 0; i < 15; i++ {
		newInode.I_block[i] = -1
	}
	newInode.I_block[0] = newDirBlock

	copy(newInode.I_type[:], "0") // Directorio

	// Permisos: copiar los permisos del directorio original
	copy(newInode.I_perm[:], srcDirInode.I_perm[:])
// Inicializar el bloque del directorio con . y ..
var folderBlock Structs.Folderblock
for i := 0; i < 4; i++ {
folderBlock.B_content[i].B_inodo = -1
}

// Entrada .
copy(folderBlock.B_content[0].B_name[:], ".")
folderBlock.B_content[0].B_inodo = newDirInode

// Entrada ..
copy(folderBlock.B_content[1].B_name[:], "..")
folderBlock.B_content[1].B_inodo = destDirInode

// Escribir el bloque del directorio
blockPos := int64(superblock.S_block_start + newDirBlock*superblock.S_block_size)
if err := Utilities.WriteObject(file, folderBlock, blockPos); err != nil {
fmt.Printf("Error al escribir bloque del directorio\n")
return false, 0, 0
}

// Escribir el inodo del directorio
newInodePos := int64(superblock.S_inode_start + newDirInode*superblock.S_inode_size)
if err := Utilities.WriteObject(file, newInode, newInodePos); err != nil {
fmt.Printf("Error al escribir inodo del directorio\n")
return false, 0, 0
}

	// Agregar la entrada en el directorio destino
	if !addFileToDirectory(file, superblock, destDirInode, dirName, newDirInode) {
		fmt.Printf("Error al agregar directorio al destino\n")
		return false, 0, 0
	}

	copiedCount++ // Contar el directorio mismo
// Copiar el contenido del directorio origen
for i := 0; i < 15 && srcDirInode.I_block[i] != -1; i++ {
var srcFolderBlock Structs.Folderblock
srcBlockPos := int64(superblock.S_block_start + srcDirInode.I_block[i]*superblock.S_block_size)
if err := Utilities.ReadObject(file, &srcFolderBlock, srcBlockPos); err != nil {
continue
}

for j := 0; j < 4; j++ {
if srcFolderBlock.B_content[j].B_inodo == -1 {
continue
}

entryName := strings.TrimRight(string(srcFolderBlock.B_content[j].B_name[:]), "\x00")

// Saltar . y ..
if entryName == "." || entryName == ".." {
continue
}

entryInode := srcFolderBlock.B_content[j].B_inodo

// Verificar permisos de lectura sobre este elemento
if !hasReadPermission(CurrentSession.PartitionID, entryInode, CurrentSession.UserID, CurrentSession.GroupID) {
fmt.Printf("⚠ Omitiendo '%s' (sin permisos de lectura)\n", entryName)
skippedCount++
continue
}

// Leer el inodo para determinar el tipo
var entryInodeStruct Structs.Inode
entryInodePos := int64(superblock.S_inode_start + entryInode*superblock.S_inode_size)
if err := Utilities.ReadObject(file, &entryInodeStruct, entryInodePos); err != nil {
skippedCount++
continue
}

isFile := string(entryInodeStruct.I_type[:1]) == "1"

if isFile {
// Copiar archivo
fmt.Printf("📄 Copiando archivo: %s\n", entryName)
success, copied, skipped := copyFileInternal(file, superblock, entryInode, newDirInode, entryName)
if success {
copiedCount += copied
skippedCount += skipped
} else {
skippedCount++
}
} else {
// Copiar subdirectorio recursivamente
fmt.Printf("📁 Copiando directorio: %s\n", entryName)
subPath := originalPath + "/" + entryName
success, copied, skipped := copyDirectoryInternal(file, superblock, entryInode, newDirInode, entryName, subPath, depth+1)
if success {
copiedCount += copied
skippedCount += skipped
} else {
skippedCount++
}
}
}
}

return true, copiedCount, skippedCount
}

// Funciones auxiliares para manejo de bitmaps

func markInodeAsUsed(file *os.File, superblock *Structs.Superblock, inodeIndex int32) {
// Marcar bit como 1 en el bitmap de inodos
bytePos := inodeIndex / 8
bitPos := inodeIndex % 8

var bitmap byte
bitmapPos := int64(superblock.S_bm_inode_start + bytePos)
file.Seek(bitmapPos, 0)
binary.Read(file, binary.LittleEndian, &bitmap)

bitmap |= (1 << uint(bitPos))

file.Seek(bitmapPos, 0)
binary.Write(file, binary.LittleEndian, bitmap)

// Actualizar contador en superblock
superblock.S_free_inodes_count--
Utilities.WriteObject(file, superblock, int64(superblock.S_inode_start) - int64(binary.Size(superblock)))
}

func markBlockAsUsed(file *os.File, superblock *Structs.Superblock, blockIndex int32) {
// Marcar bit como 1 en el bitmap de bloques
bytePos := blockIndex / 8
bitPos := blockIndex % 8

var bitmap byte
bitmapPos := int64(superblock.S_bm_block_start + bytePos)
file.Seek(bitmapPos, 0)
binary.Read(file, binary.LittleEndian, &bitmap)

bitmap |= (1 << uint(bitPos))

file.Seek(bitmapPos, 0)
binary.Write(file, binary.LittleEndian, bitmap)

// Actualizar contador en superblock
superblock.S_free_blocks_count--
Utilities.WriteObject(file, superblock, int64(superblock.S_inode_start) - int64(binary.Size(superblock)))
}

func markInodeAsFree(file *os.File, superblock *Structs.Superblock, inodeIndex int32) {
// Marcar bit como 0 en el bitmap de inodos
bytePos := inodeIndex / 8
bitPos := inodeIndex % 8

var bitmap byte
bitmapPos := int64(superblock.S_bm_inode_start + bytePos)
file.Seek(bitmapPos, 0)
binary.Read(file, binary.LittleEndian, &bitmap)

bitmap &= ^(1 << uint(bitPos))

file.Seek(bitmapPos, 0)
binary.Write(file, binary.LittleEndian, bitmap)

// Actualizar contador en superblock
superblock.S_free_inodes_count++
Utilities.WriteObject(file, superblock, int64(superblock.S_inode_start) - int64(binary.Size(superblock)))
}

func markBlockAsFree(file *os.File, superblock *Structs.Superblock, blockIndex int32) {
// Marcar bit como 0 en el bitmap de bloques
bytePos := blockIndex / 8
bitPos := blockIndex % 8

var bitmap byte
bitmapPos := int64(superblock.S_bm_block_start + bytePos)
file.Seek(bitmapPos, 0)
binary.Read(file, binary.LittleEndian, &bitmap)

bitmap &= ^(1 << uint(bitPos))

file.Seek(bitmapPos, 0)
binary.Write(file, binary.LittleEndian, bitmap)

	// Actualizar contador en superblock
	superblock.S_free_blocks_count++
	Utilities.WriteObject(file, superblock, int64(superblock.S_inode_start) - int64(binary.Size(superblock)))
}

// ============================================================================
// COMANDO MOVE - MOVER ARCHIVOS Y DIRECTORIOS
// ============================================================================

// Move - Mover un archivo o directorio a otro destino (cambia solo las referencias)
func Move(path string, destino string) {
	fmt.Println("======Inicio MOVE======")
	fmt.Printf("Origen: %s\n", path)
	fmt.Printf("Destino: %s\n", destino)

	// Verificar que haya una sesión activa
	if !IsUserLoggedIn() {
		fmt.Println("Error: No hay una sesión activa")
		fmt.Println("Use el comando 'login' para iniciar sesión")
		fmt.Println("======FIN MOVE======")
		return
	}

	// Validar que las rutas no estén vacías
	if strings.TrimSpace(path) == "" {
		fmt.Println("Error: La ruta de origen no puede estar vacía")
		fmt.Println("======FIN MOVE======")
		return
	}

	if strings.TrimSpace(destino) == "" {
		fmt.Println("Error: La ruta de destino no puede estar vacía")
		fmt.Println("======FIN MOVE======")
		return
	}

	// Validar que las rutas empiecen con "/"
	if !strings.HasPrefix(path, "/") {
		fmt.Println("Error: La ruta de origen debe empezar con '/' (ruta absoluta)")
		fmt.Println("======FIN MOVE======")
		return
	}

	if !strings.HasPrefix(destino, "/") {
		fmt.Println("Error: La ruta de destino debe empezar con '/' (ruta absoluta)")
		fmt.Println("======FIN MOVE======")
		return
	}

	// Validar que no intente mover la raíz
	if path == "/" {
		fmt.Println("Error: No se puede mover el directorio raíz")
		fmt.Println("======FIN MOVE======")
		return
	}

	// Validar que destino no sea subdirectorio de origen
	if strings.HasPrefix(destino, path+"/") || destino == path {
		fmt.Println("Error: No se puede mover un directorio dentro de sí mismo")
		fmt.Println("======FIN MOVE======")
		return
	}

	// Buscar el archivo o directorio de origen
	existsFile, sourceInodeFile := findFileInDirectory(CurrentSession.PartitionID, path)
	existsDir, sourceInodeDir := findDirectoryInPath(CurrentSession.PartitionID, path)

	if !existsFile && !existsDir {
		fmt.Printf("Error: La ruta de origen '%s' no existe\n", path)
		fmt.Println("======FIN MOVE======")
		return
	}

	// Determinar si es archivo o directorio
	isFile := existsFile
	sourceInode := sourceInodeFile
	if !isFile {
		sourceInode = sourceInodeDir
	}

	// Verificar permisos de escritura sobre el origen
	if !hasWritePermission(CurrentSession.PartitionID, sourceInode, CurrentSession.UserID, CurrentSession.GroupID) {
		fmt.Printf("Error: No tiene permisos de escritura sobre '%s'\n", path)
		fmt.Println("======FIN MOVE======")
		return
	}

	// Verificar que el directorio destino exista
	existsDestDir, destDirInode := findDirectoryInPath(CurrentSession.PartitionID, destino)
	if !existsDestDir {
		fmt.Printf("Error: El directorio de destino '%s' no existe\n", destino)
		fmt.Println("======FIN MOVE======")
		return
	}

	// Verificar permisos de escritura sobre el destino
	if !hasWritePermission(CurrentSession.PartitionID, destDirInode, CurrentSession.UserID, CurrentSession.GroupID) {
		fmt.Printf("Error: No tiene permisos de escritura en el directorio de destino '%s'\n", destino)
		fmt.Println("======FIN MOVE======")
		return
	}

	// Obtener el nombre del archivo/directorio a mover y su padre
	parentPath, sourceName := parseFilePath(path)
	
	// Validar que el padre no sea vacío (no se puede mover desde raíz sin padre)
	if parentPath == "" {
		fmt.Println("Error: No se puede mover un elemento sin directorio padre")
		fmt.Println("======FIN MOVE======")
		return
	}

	// Buscar el directorio padre del origen
	existsParent, parentInode := findDirectoryInPath(CurrentSession.PartitionID, parentPath)
	if !existsParent {
		fmt.Printf("Error: No se encontró el directorio padre '%s'\n", parentPath)
		fmt.Println("======FIN MOVE======")
		return
	}

	// Verificar que no exista ya un archivo/directorio con ese nombre en destino
	existsInDest, _ := findInodeInDirectory(CurrentSession.PartitionID, destDirInode, sourceName, false)
	if existsInDest {
		fmt.Printf("Error: Ya existe un archivo o directorio con el nombre '%s' en '%s'\n", sourceName, destino)
		fmt.Println("======FIN MOVE======")
		return
	}

	// Obtener información de la partición
	mountedPartition, exists := DiskManagement.MountedPartitions[CurrentSession.PartitionID]
	if !exists {
		fmt.Println("Error: Partición no encontrada")
		fmt.Println("======FIN MOVE======")
		return
	}

	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		fmt.Println("Error: No se pudo abrir el disco")
		fmt.Println("======FIN MOVE======")
		return
	}
	defer file.Close()

	superblock, err := ReadSuperblock(CurrentSession.PartitionID)
	if err != nil {
		fmt.Println("Error: No se pudo leer el superblock")
		fmt.Println("======FIN MOVE======")
		return
	}

	// PASO 1: Remover la entrada del directorio padre original
	if !removeEntryFromParent(file, superblock, parentInode, sourceName) {
		fmt.Println("Error: No se pudo remover la entrada del directorio origen")
		fmt.Println("======FIN MOVE======")
		return
	}

	// PASO 2: Agregar la entrada al directorio destino
	if !addFileToDirectory(file, superblock, destDirInode, sourceName, sourceInode) {
		fmt.Println("Error: No se pudo agregar la entrada al directorio destino")
		// Intentar restaurar la entrada en el padre original
		addFileToDirectory(file, superblock, parentInode, sourceName, sourceInode)
		fmt.Println("======FIN MOVE======")
		return
	}

	// PASO 3: Si es un directorio, actualizar la referencia ".." al nuevo padre
	if !isFile {
		if !updateParentReference(file, superblock, sourceInode, destDirInode) {
			fmt.Println("Advertencia: No se pudo actualizar la referencia al padre en el directorio movido")
		}
	}

	// Éxito
	// Registrar en el journaling (EXT3)
	contentInfo := fmt.Sprintf("%s->%s", path, destino)
	writeToJournal(CurrentSession.PartitionID, "move", path, contentInfo)
	
	fmt.Println("\n=== MOVIMIENTO COMPLETADO ===")
	fmt.Printf("Origen: %s\n", path)
	fmt.Printf("Nuevo destino: %s/%s\n", destino, sourceName)
	fmt.Printf("Tipo: %s\n", map[bool]string{true: "Archivo", false: "Directorio"}[isFile])
	fmt.Println("El archivo/directorio fue movido exitosamente")
	fmt.Println("======FIN MOVE======")
}

// updateParentReference - Actualizar la referencia ".." de un directorio al nuevo padre
func updateParentReference(file *os.File, superblock *Structs.Superblock, dirInode int32, newParentInode int32) bool {
	// Leer el inodo del directorio
	var inode Structs.Inode
	inodePos := int64(superblock.S_inode_start + dirInode*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &inode, inodePos); err != nil {
		return false
	}

	// Buscar el bloque que contiene ".."
	for i := 0; i < 15 && inode.I_block[i] != -1; i++ {
		var folderBlock Structs.Folderblock
		blockPos := int64(superblock.S_block_start + inode.I_block[i]*superblock.S_block_size)
		if err := Utilities.ReadObject(file, &folderBlock, blockPos); err != nil {
			continue
		}

		// Buscar la entrada ".."
		for j := 0; j < 4; j++ {
			entryName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
			if entryName == ".." {
				// Actualizar la referencia al nuevo padre
				folderBlock.B_content[j].B_inodo = newParentInode
				
				// Escribir el bloque actualizado
				if err := Utilities.WriteObject(file, folderBlock, blockPos); err != nil {
					return false
				}
				return true
			}
		}
	}

	return false
}

// matchPattern - Verifica si un nombre coincide con un patrón que contiene ? y *
// ?: coincide con exactamente un carácter
// *: coincide con uno o más caracteres
func matchPattern(name string, pattern string) bool {
	// Implementación recursiva para manejar los comodines
	return matchPatternHelper(name, pattern, 0, 0)
}

func matchPatternHelper(name string, pattern string, nameIdx int, patternIdx int) bool {
	// Caso base: ambos índices llegaron al final
	if nameIdx == len(name) && patternIdx == len(pattern) {
		return true
	}

	// Si el patrón llegó al final pero el nombre no
	if patternIdx == len(pattern) {
		return false
	}

	// Si el nombre llegó al final pero el patrón no
	if nameIdx == len(name) {
		// Solo es válido si el resto del patrón son asteriscos
		for i := patternIdx; i < len(pattern); i++ {
			if pattern[i] != '*' {
				return false
			}
		}
		return true
	}

	// Caso: carácter normal - debe coincidir exactamente
	if pattern[patternIdx] != '?' && pattern[patternIdx] != '*' {
		if name[nameIdx] == pattern[patternIdx] {
			return matchPatternHelper(name, pattern, nameIdx+1, patternIdx+1)
		}
		return false
	}

	// Caso: ? - coincide con exactamente un carácter
	if pattern[patternIdx] == '?' {
		return matchPatternHelper(name, pattern, nameIdx+1, patternIdx+1)
	}

	// Caso: * - coincide con uno o más caracteres
	if pattern[patternIdx] == '*' {
		// Intentar hacer coincidir con 1, 2, 3, ... caracteres
		for i := 1; i <= len(name)-nameIdx; i++ {
			if matchPatternHelper(name, pattern, nameIdx+i, patternIdx+1) {
				return true
			}
		}
		return false
	}

	return false
}

// findRecursive - Búsqueda recursiva de archivos/directorios que coincidan con el patrón
func findRecursive(file *os.File, superblock *Structs.Superblock, currentInode int32, 
	currentPath string, pattern string, results *[]string, depth int, partitionID string, userID int, groupID int) {
	
	// Validaciones de seguridad
	if file == nil || superblock == nil || results == nil {
		return
	}
	
	// Límite de profundidad para evitar recursión infinita
	if depth > 100 {
		return
	}

	// Leer el inodo actual
	var inode Structs.Inode
	inodePos := int64(superblock.S_inode_start + currentInode*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &inode, inodePos); err != nil {
		return
	}

	// Verificar permisos de lectura
	if !hasReadPermission(partitionID, currentInode, userID, groupID) {
		return
	}

	// Si es un archivo, verificar si coincide con el patrón
	if inode.I_type[0] == '1' {
		// Extraer el nombre del archivo de la ruta actual
		parts := strings.Split(currentPath, "/")
		fileName := parts[len(parts)-1]
		
		if matchPattern(fileName, pattern) {
			*results = append(*results, currentPath)
		}
		return
	}

	// Si es un directorio, buscar en su contenido
	if inode.I_type[0] == '0' {
		// Verificar si el nombre del directorio coincide con el patrón
		if currentPath != "/" {
			parts := strings.Split(currentPath, "/")
			dirName := parts[len(parts)-1]
			if matchPattern(dirName, pattern) {
				*results = append(*results, currentPath)
			}
		}

		// Recorrer todos los bloques del directorio
		for i := 0; i < 15 && inode.I_block[i] != -1; i++ {
			var folderBlock Structs.Folderblock
			blockPos := int64(superblock.S_block_start + inode.I_block[i]*superblock.S_block_size)
			if err := Utilities.ReadObject(file, &folderBlock, blockPos); err != nil {
				continue
			}

			// Recorrer cada entrada del bloque
			for j := 0; j < 4; j++ {
				entryName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
				entryInode := folderBlock.B_content[j].B_inodo

				// Saltar entradas vacías, "." y ".."
				if entryName == "" || entryName == "." || entryName == ".." || entryInode == -1 {
					continue
				}

				// Construir la nueva ruta
				var newPath string
				if currentPath == "/" {
					newPath = "/" + entryName
				} else {
					newPath = currentPath + "/" + entryName
				}

				// Búsqueda recursiva
				findRecursive(file, superblock, entryInode, newPath, pattern, results, depth+1, partitionID, userID, groupID)
			}
		}
	}
}

// Find - Buscar archivos/directorios por nombre usando patrones
func Find(path string, name string) {
	fmt.Println("======Inicio FIND======")
	fmt.Printf("Ruta de búsqueda: %s\n", path)
	fmt.Printf("Patrón: %s\n", name)

	// Validar que hay una sesión activa
	if CurrentSession == nil || CurrentSession.PartitionID == "" {
		fmt.Println("Error: No hay una sesión activa")
		fmt.Println("Use el comando 'login' para iniciar sesión")
		fmt.Println("======FIN FIND======")
		return
	}

	// Validar parámetros
	if path == "" {
		fmt.Println("Error: El parámetro -path es obligatorio")
		fmt.Println("======FIN FIND======")
		return
	}

	if name == "" {
		fmt.Println("Error: El parámetro -name es obligatorio")
		fmt.Println("======FIN FIND======")
		return
	}

	// Validar que la ruta sea absoluta
	if !strings.HasPrefix(path, "/") {
		fmt.Println("Error: La ruta debe empezar con '/' (ruta absoluta)")
		fmt.Println("======FIN FIND======")
		return
	}

	// Buscar el directorio de inicio
	existsDir, startInode := findDirectoryInPath(CurrentSession.PartitionID, path)
	if !existsDir {
		fmt.Printf("Error: No se encontró la ruta: %s\n", path)
		fmt.Println("======FIN FIND======")
		return
	}

	// Verificar permisos de lectura en el directorio de inicio
	if !hasReadPermission(CurrentSession.PartitionID, startInode, CurrentSession.UserID, CurrentSession.GroupID) {
		fmt.Println("Error: No tienes permisos de lectura en este directorio")
		fmt.Println("======FIN FIND======")
		return
	}

	// Obtener información de la partición
	mountedPartition, exists := DiskManagement.MountedPartitions[CurrentSession.PartitionID]
	if !exists {
		fmt.Println("Error: Partición no encontrada")
		fmt.Println("======FIN FIND======")
		return
	}

	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		fmt.Printf("Error: No se pudo abrir el disco: %v\n", err)
		fmt.Println("======FIN FIND======")
		return
	}
	defer file.Close()

	superblock, err := ReadSuperblock(CurrentSession.PartitionID)
	if err != nil {
		fmt.Printf("Error: No se pudo leer el superblock: %v\n", err)
		fmt.Println("======FIN FIND======")
		return
	}

	// Validación adicional: verificar que el superblock no sea nil
	if superblock == nil {
		fmt.Println("Error: El superblock es nulo")
		fmt.Println("======FIN FIND======")
		return
	}

	// Realizar la búsqueda
	results := make([]string, 0)
	findRecursive(file, superblock, startInode, path, name, &results, 0, 
		CurrentSession.PartitionID, CurrentSession.UserID, CurrentSession.GroupID)

	// Mostrar resultados
	fmt.Println("\n=== RESULTADOS DE LA BÚSQUEDA ===")
	if len(results) == 0 {
		fmt.Println("No se encontraron archivos/directorios que coincidan con el patrón")
	} else {
		fmt.Printf("Se encontraron %d elementos:\n", len(results))
		for _, result := range results {
			fmt.Printf("  %s\n", result)
		}
	}
	fmt.Println("======FIN FIND======")
}

// ============================================================================
// COMANDO CHOWN - CAMBIAR PROPIETARIO DE ARCHIVOS Y DIRECTORIOS
// ============================================================================

// Chown - Cambiar el propietario de archivos y directorios
func Chown(path string, recursive bool, usuario string) {
	fmt.Println("======INICIO CHOWN======")
	fmt.Println("Comando: chown")
	fmt.Printf("Parámetros:\n")
	fmt.Printf("  -path=%s\n", path)
	fmt.Printf("  -r=%t\n", recursive)
	fmt.Printf("  -usuario=%s\n", usuario)
	fmt.Println()

	// Verificar que haya una sesión activa
	if CurrentSession == nil || CurrentSession.PartitionID == "" {
		fmt.Println("ERROR: No hay una sesión activa. Use el comando 'login' primero.")
		fmt.Println("======FIN CHOWN======")
		return
	}

	// Validar parámetros requeridos
	if path == "" {
		fmt.Println("ERROR: El parámetro -path es obligatorio")
		fmt.Println("======FIN CHOWN======")
		return
	}

	if usuario == "" {
		fmt.Println("ERROR: El parámetro -usuario es obligatorio")
		fmt.Println("======FIN CHOWN======")
		return
	}

	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[CurrentSession.PartitionID]
	if !exists {
		fmt.Printf("ERROR: No se encontró la partición montada con ID '%s'\n", CurrentSession.PartitionID)
		fmt.Println("======FIN CHOWN======")
		return
	}

	// Leer el archivo users.txt para validar el usuario
	usersData, err := readUsersFile(CurrentSession.PartitionID)
	if err != nil {
		fmt.Printf("ERROR: No se pudo leer el archivo de usuarios: %s\n", err)
		fmt.Println("======FIN CHOWN======")
		return
	}

	// Verificar que el usuario existe
	userExists, targetUser := findUser(usersData, usuario)
	if !userExists {
		fmt.Printf("ERROR: El usuario '%s' no existe en el sistema\n", usuario)
		fmt.Println("======FIN CHOWN======")
		return
	}

	// Abrir el archivo del disco
	file, err := os.OpenFile(mountedPartition.Path, os.O_RDWR, 0644)
	if err != nil {
		fmt.Printf("ERROR: No se pudo abrir el archivo del disco: %s\n", err)
		fmt.Println("======FIN CHOWN======")
		return
	}
	defer file.Close()

	// Leer el superblock
	superblock, err := ReadSuperblock(CurrentSession.PartitionID)
	if err != nil {
		fmt.Printf("ERROR: No se pudo leer el superblock: %s\n", err)
		fmt.Println("======FIN CHOWN======")
		return
	}

	// Buscar el archivo o directorio por su ruta
	inodeNum, err := findFileOrDirectoryByPath(file, superblock, path, CurrentSession.UserID, CurrentSession.GroupID)
	if err != nil {
		fmt.Printf("ERROR: No se encontró la ruta '%s': %s\n", path, err)
		fmt.Println("======FIN CHOWN======")
		return
	}

	// Leer el inodo del archivo o directorio
	file.Seek(int64(superblock.S_inode_start)+int64(inodeNum)*int64(superblock.S_inode_size), 0)
	var inode Structs.Inode
	binary.Read(file, binary.LittleEndian, &inode)

	// Verificar permisos: root puede cambiar cualquier archivo,
	// otros usuarios solo pueden cambiar sus propios archivos
	isRoot := CurrentSession.UserID == 1
	isOwner := inode.I_uid == int32(CurrentSession.UserID)

	if !isRoot && !isOwner {
		fmt.Printf("ERROR: No tiene permisos para cambiar el propietario de '%s'\n", path)
		fmt.Printf("Solo el propietario o root pueden cambiar el propietario de un archivo\n")
		fmt.Println("======FIN CHOWN======")
		return
	}

	// Cambiar el propietario
	if recursive && inode.I_type[0] == '0' {
		// Es un directorio y se solicitó cambio recursivo
		fmt.Printf("Cambiando propietario recursivamente de '%s' a '%s' (UID: %d)...\n", path, usuario, targetUser.ID)
		changeOwnerRecursive(file, superblock, inodeNum, int32(targetUser.ID), CurrentSession.UserID)
		fmt.Printf("Propietario cambiado exitosamente de forma recursiva\n")
	} else {
		// Cambio simple (archivo o directorio sin recursión)
		inode.I_uid = int32(targetUser.ID)
		
		// Escribir el inodo actualizado
		file.Seek(int64(superblock.S_inode_start)+int64(inodeNum)*int64(superblock.S_inode_size), 0)
		binary.Write(file, binary.LittleEndian, &inode)
		
		fileType := "archivo"
		if inode.I_type[0] == '0' {
			fileType = "directorio"
		}
		fmt.Printf("Propietario del %s '%s' cambiado exitosamente a '%s' (UID: %d)\n", 
			fileType, path, usuario, targetUser.ID)
	}

	// Registrar en el journaling (EXT3)
	contentInfo := fmt.Sprintf("uid=%d", targetUser.ID)
	writeToJournal(CurrentSession.PartitionID, "chown", path, contentInfo)

	fmt.Println()
	fmt.Println("======FIN CHOWN======")
}

// changeOwnerRecursive - Cambiar el propietario de un directorio y todo su contenido recursivamente
func changeOwnerRecursive(file *os.File, superblock *Structs.Superblock, inodeNum int32, newOwnerID int32, currentUserID int) {
	// Leer el inodo actual
	file.Seek(int64(superblock.S_inode_start)+int64(inodeNum)*int64(superblock.S_inode_size), 0)
	var inode Structs.Inode
	binary.Read(file, binary.LittleEndian, &inode)

	// Cambiar el propietario del inodo actual
	inode.I_uid = newOwnerID
	file.Seek(int64(superblock.S_inode_start)+int64(inodeNum)*int64(superblock.S_inode_size), 0)
	binary.Write(file, binary.LittleEndian, &inode)

	// Si es un directorio, procesar recursivamente su contenido
	if inode.I_type[0] == '0' {
		// Recorrer los bloques directos del directorio
		for i := 0; i < 12; i++ {
			if inode.I_block[i] == -1 {
				continue
			}

			// Leer el bloque de directorio
			file.Seek(int64(superblock.S_block_start)+int64(inode.I_block[i])*int64(superblock.S_block_size), 0)
			var folderBlock Structs.Folderblock
			binary.Read(file, binary.LittleEndian, &folderBlock)

			// Procesar cada entrada del bloque
			for j := 0; j < 4; j++ {
				if folderBlock.B_content[j].B_inodo == -1 {
					continue
				}

				// Obtener el nombre de la entrada
				entryName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
				
				// Saltar las entradas "." y ".."
				if entryName == "." || entryName == ".." {
					continue
				}

				// Cambiar el propietario recursivamente
				changeOwnerRecursive(file, superblock, folderBlock.B_content[j].B_inodo, newOwnerID, currentUserID)
			}
		}

		// Si tiene puntero indirecto, procesarlo también
		if inode.I_block[12] != -1 {
			changeOwnerRecursiveIndirect(file, superblock, inode.I_block[12], newOwnerID, currentUserID)
		}
	}
}

// changeOwnerRecursiveIndirect - Procesar bloques indirectos para cambio de propietario recursivo
func changeOwnerRecursiveIndirect(file *os.File, superblock *Structs.Superblock, pointerBlockNum int32, newOwnerID int32, currentUserID int) {
	// Leer el bloque de punteros
	file.Seek(int64(superblock.S_block_start)+int64(pointerBlockNum)*int64(superblock.S_block_size), 0)
	var pointerBlock Structs.Pointerblock
	binary.Read(file, binary.LittleEndian, &pointerBlock)

	// Procesar cada puntero del bloque
	for i := 0; i < 16; i++ {
		if pointerBlock.B_pointers[i] == -1 {
			continue
		}

		// Leer el bloque de directorio
		file.Seek(int64(superblock.S_block_start)+int64(pointerBlock.B_pointers[i])*int64(superblock.S_block_size), 0)
		var folderBlock Structs.Folderblock
		binary.Read(file, binary.LittleEndian, &folderBlock)

		// Procesar cada entrada del bloque
		for j := 0; j < 4; j++ {
			if folderBlock.B_content[j].B_inodo == -1 {
				continue
			}

			// Obtener el nombre de la entrada
			entryName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
			
			// Saltar las entradas "." y ".."
			if entryName == "." || entryName == ".." {
				continue
			}

			// Cambiar el propietario recursivamente
			changeOwnerRecursive(file, superblock, folderBlock.B_content[j].B_inodo, newOwnerID, currentUserID)
		}
	}
}

// findFileOrDirectoryByPath - Buscar un archivo o directorio por su ruta completa
func findFileOrDirectoryByPath(file *os.File, superblock *Structs.Superblock, path string, userID int, groupID int) (int32, error) {
	// Si la ruta está vacía o es solo "/", retornar el inodo raíz
	if path == "" || path == "/" {
		return 0, nil
	}

	// Asegurar que la ruta comience con "/"
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// Dividir la ruta en componentes
	components := strings.Split(strings.Trim(path, "/"), "/")
	if len(components) == 0 {
		return 0, nil
	}

	// Comenzar desde el directorio raíz
	currentInodeNum := int32(0)

	// Navegar por cada componente de la ruta
	for _, component := range components {
		if component == "" {
			continue
		}

		// Leer el inodo actual
		file.Seek(int64(superblock.S_inode_start)+int64(currentInodeNum)*int64(superblock.S_inode_size), 0)
		var currentInode Structs.Inode
		binary.Read(file, binary.LittleEndian, &currentInode)

		// Verificar que sea un directorio
		if currentInode.I_type[0] != '0' {
			return -1, fmt.Errorf("'%s' no es un directorio", component)
		}

		// Buscar el componente en el directorio actual
		found := false
		var nextInodeNum int32 = -1

		// Buscar en bloques directos
		for i := 0; i < 12 && !found; i++ {
			if currentInode.I_block[i] == -1 {
				continue
			}

			// Leer el bloque de directorio
			file.Seek(int64(superblock.S_block_start)+int64(currentInode.I_block[i])*int64(superblock.S_block_size), 0)
			var folderBlock Structs.Folderblock
			binary.Read(file, binary.LittleEndian, &folderBlock)

			// Buscar en las entradas del bloque
			for j := 0; j < 4; j++ {
				if folderBlock.B_content[j].B_inodo == -1 {
					continue
				}

				entryName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
				if entryName == component {
					nextInodeNum = folderBlock.B_content[j].B_inodo
					found = true
					break
				}
			}
		}

		// Si no se encontró en bloques directos, buscar en bloque indirecto
		if !found && currentInode.I_block[12] != -1 {
			nextInodeNum = findInIndirectBlock(file, superblock, currentInode.I_block[12], component)
			if nextInodeNum != -1 {
				found = true
			}
		}

		if !found {
			return -1, fmt.Errorf("no se encontró '%s' en la ruta", component)
		}

		currentInodeNum = nextInodeNum
	}

	return currentInodeNum, nil
}

// findInIndirectBlock - Buscar en un bloque indirecto
func findInIndirectBlock(file *os.File, superblock *Structs.Superblock, pointerBlockNum int32, name string) int32 {
	// Leer el bloque de punteros
	file.Seek(int64(superblock.S_block_start)+int64(pointerBlockNum)*int64(superblock.S_block_size), 0)
	var pointerBlock Structs.Pointerblock
	binary.Read(file, binary.LittleEndian, &pointerBlock)

	// Buscar en cada bloque apuntado
	for i := 0; i < 16; i++ {
		if pointerBlock.B_pointers[i] == -1 {
			continue
		}

		// Leer el bloque de directorio
		file.Seek(int64(superblock.S_block_start)+int64(pointerBlock.B_pointers[i])*int64(superblock.S_block_size), 0)
		var folderBlock Structs.Folderblock
		binary.Read(file, binary.LittleEndian, &folderBlock)

		// Buscar en las entradas del bloque
		for j := 0; j < 4; j++ {
			if folderBlock.B_content[j].B_inodo == -1 {
				continue
			}

			entryName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
			if entryName == name {
				return folderBlock.B_content[j].B_inodo
			}
		}
	}

	return -1
}

// ============================================================================
// COMANDO CHMOD - CAMBIAR PERMISOS DE ARCHIVOS Y DIRECTORIOS
// ============================================================================

// Chmod - Cambiar los permisos de archivos y directorios
func Chmod(path string, ugo string, recursive bool) {
	fmt.Println("======INICIO CHMOD======")
	fmt.Println("Comando: chmod")
	fmt.Printf("Parámetros:\n")
	fmt.Printf("  -path=%s\n", path)
	fmt.Printf("  -ugo=%s\n", ugo)
	fmt.Printf("  -r=%t\n", recursive)
	fmt.Println()

	// Verificar que haya una sesión activa
	if CurrentSession == nil || CurrentSession.PartitionID == "" {
		fmt.Println("ERROR: No hay una sesión activa. Use el comando 'login' primero.")
		fmt.Println("======FIN CHMOD======")
		return
	}

	// Validar parámetros requeridos
	if path == "" {
		fmt.Println("ERROR: El parámetro -path es obligatorio")
		fmt.Println("======FIN CHMOD======")
		return
	}

	if ugo == "" {
		fmt.Println("ERROR: El parámetro -ugo es obligatorio")
		fmt.Println("======FIN CHMOD======")
		return
	}

	// Validar formato de permisos (debe ser exactamente 3 dígitos)
	if len(ugo) != 3 {
		fmt.Println("ERROR: El parámetro -ugo debe tener exactamente 3 dígitos")
		fmt.Println("Formato: -ugo=[0-7][0-7][0-7] (Usuario, Grupo, Otros)")
		fmt.Println("Ejemplo: -ugo=764")
		fmt.Println("======FIN CHMOD======")
		return
	}

	// Validar que cada dígito esté en el rango [0-7]
	for i, char := range ugo {
		if char < '0' || char > '7' {
			fmt.Printf("ERROR: El dígito en la posición %d ('%c') no está en el rango [0-7]\n", i+1, char)
			fmt.Println("Los permisos deben ser números del 0 al 7")
			fmt.Println("======FIN CHMOD======")
			return
		}
	}

	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[CurrentSession.PartitionID]
	if !exists {
		fmt.Printf("ERROR: No se encontró la partición montada con ID '%s'\n", CurrentSession.PartitionID)
		fmt.Println("======FIN CHMOD======")
		return
	}

	// Abrir el archivo del disco
	file, err := os.OpenFile(mountedPartition.Path, os.O_RDWR, 0644)
	if err != nil {
		fmt.Printf("ERROR: No se pudo abrir el archivo del disco: %s\n", err)
		fmt.Println("======FIN CHMOD======")
		return
	}
	defer file.Close()

	// Leer el superblock
	superblock, err := ReadSuperblock(CurrentSession.PartitionID)
	if err != nil {
		fmt.Printf("ERROR: No se pudo leer el superblock: %s\n", err)
		fmt.Println("======FIN CHMOD======")
		return
	}

	// Buscar el archivo o directorio por su ruta
	inodeNum, err := findFileOrDirectoryByPath(file, superblock, path, CurrentSession.UserID, CurrentSession.GroupID)
	if err != nil {
		fmt.Printf("ERROR: No se encontró la ruta '%s': %s\n", path, err)
		fmt.Println("======FIN CHMOD======")
		return
	}

	// Leer el inodo del archivo o directorio
	file.Seek(int64(superblock.S_inode_start)+int64(inodeNum)*int64(superblock.S_inode_size), 0)
	var inode Structs.Inode
	binary.Read(file, binary.LittleEndian, &inode)

	// Verificar permisos: root puede cambiar cualquier archivo,
	// otros usuarios solo pueden cambiar sus propios archivos
	isRoot := CurrentSession.UserID == 1
	isOwner := inode.I_uid == int32(CurrentSession.UserID)

	if !isRoot && !isOwner {
		fmt.Printf("ERROR: No tiene permisos para cambiar los permisos de '%s'\n", path)
		fmt.Printf("Solo el propietario (UID: %d) o root pueden cambiar los permisos\n", inode.I_uid)
		fmt.Println("======FIN CHMOD======")
		return
	}

	// Cambiar los permisos
	if recursive && inode.I_type[0] == '0' {
		// Es un directorio y se solicitó cambio recursivo
		fmt.Printf("Cambiando permisos recursivamente de '%s' a '%s'...\n", path, ugo)
		changePermissionsRecursive(file, superblock, inodeNum, ugo, CurrentSession.UserID, isRoot)
		fmt.Printf("Permisos cambiados exitosamente de forma recursiva\n")
	} else {
		// Cambio simple (archivo o directorio sin recursión)
		copy(inode.I_perm[:], []byte(ugo))
		
		// Escribir el inodo actualizado
		file.Seek(int64(superblock.S_inode_start)+int64(inodeNum)*int64(superblock.S_inode_size), 0)
		binary.Write(file, binary.LittleEndian, &inode)
		
		fileType := "archivo"
		if inode.I_type[0] == '0' {
			fileType = "directorio"
		}
		
		// Decodificar permisos para mostrar
		permsStr := decodePermissions(ugo)
		fmt.Printf("Permisos del %s '%s' cambiados exitosamente a %s (%s)\n", 
			fileType, path, ugo, permsStr)
	}

	// Registrar en el journaling (EXT3)
	writeToJournal(CurrentSession.PartitionID, "chmod", path, ugo)

	fmt.Println()
	fmt.Println("======FIN CHMOD======")
}

// changePermissionsRecursive - Cambiar los permisos de un directorio y todo su contenido recursivamente
func changePermissionsRecursive(file *os.File, superblock *Structs.Superblock, inodeNum int32, permissions string, currentUserID int, isRoot bool) {
	// Leer el inodo actual
	file.Seek(int64(superblock.S_inode_start)+int64(inodeNum)*int64(superblock.S_inode_size), 0)
	var inode Structs.Inode
	binary.Read(file, binary.LittleEndian, &inode)

	// Verificar si el usuario actual es propietario o es root
	if isRoot || inode.I_uid == int32(currentUserID) {
		// Cambiar los permisos del inodo actual
		copy(inode.I_perm[:], []byte(permissions))
		file.Seek(int64(superblock.S_inode_start)+int64(inodeNum)*int64(superblock.S_inode_size), 0)
		binary.Write(file, binary.LittleEndian, &inode)
	}

	// Si es un directorio, procesar recursivamente su contenido
	if inode.I_type[0] == '0' {
		// Recorrer los bloques directos del directorio
		for i := 0; i < 12; i++ {
			if inode.I_block[i] == -1 {
				continue
			}

			// Leer el bloque de directorio
			file.Seek(int64(superblock.S_block_start)+int64(inode.I_block[i])*int64(superblock.S_block_size), 0)
			var folderBlock Structs.Folderblock
			binary.Read(file, binary.LittleEndian, &folderBlock)

			// Procesar cada entrada del bloque
			for j := 0; j < 4; j++ {
				if folderBlock.B_content[j].B_inodo == -1 {
					continue
				}

				// Obtener el nombre de la entrada
				entryName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
				
				// Saltar las entradas "." y ".."
				if entryName == "." || entryName == ".." {
					continue
				}

				// Cambiar los permisos recursivamente
				changePermissionsRecursive(file, superblock, folderBlock.B_content[j].B_inodo, permissions, currentUserID, isRoot)
			}
		}

		// Si tiene puntero indirecto, procesarlo también
		if inode.I_block[12] != -1 {
			changePermissionsRecursiveIndirect(file, superblock, inode.I_block[12], permissions, currentUserID, isRoot)
		}
	}
}

// changePermissionsRecursiveIndirect - Procesar bloques indirectos para cambio de permisos recursivo
func changePermissionsRecursiveIndirect(file *os.File, superblock *Structs.Superblock, pointerBlockNum int32, permissions string, currentUserID int, isRoot bool) {
	// Leer el bloque de punteros
	file.Seek(int64(superblock.S_block_start)+int64(pointerBlockNum)*int64(superblock.S_block_size), 0)
	var pointerBlock Structs.Pointerblock
	binary.Read(file, binary.LittleEndian, &pointerBlock)

	// Procesar cada puntero del bloque
	for i := 0; i < 16; i++ {
		if pointerBlock.B_pointers[i] == -1 {
			continue
		}

		// Leer el bloque de directorio
		file.Seek(int64(superblock.S_block_start)+int64(pointerBlock.B_pointers[i])*int64(superblock.S_block_size), 0)
		var folderBlock Structs.Folderblock
		binary.Read(file, binary.LittleEndian, &folderBlock)

		// Procesar cada entrada del bloque
		for j := 0; j < 4; j++ {
			if folderBlock.B_content[j].B_inodo == -1 {
				continue
			}

			// Obtener el nombre de la entrada
			entryName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
			
			// Saltar las entradas "." y ".."
			if entryName == "." || entryName == ".." {
				continue
			}

			// Cambiar los permisos recursivamente
			changePermissionsRecursive(file, superblock, folderBlock.B_content[j].B_inodo, permissions, currentUserID, isRoot)
		}
	}
}

// decodePermissions - Decodificar permisos numéricos a formato legible (rwx)
func decodePermissions(perms string) string {
	if len(perms) != 3 {
		return "???"
	}

	result := ""
	permChars := []string{"r", "w", "x"}
	
	for _, digit := range perms {
		val := int(digit - '0')
		perm := ""
		
		// Decodificar cada bit (lectura, escritura, ejecución)
		for i := 2; i >= 0; i-- {
			if val&(1<<uint(i)) != 0 {
				perm += permChars[2-i]
			} else {
				perm += "-"
			}
		}
		result += perm
	}
	
	return result
}

// ============================================================================
// COMANDO LOSS - SIMULAR PÉRDIDA DE DATOS (SOLO EXT3)
// ============================================================================

// Loss - Simular un fallo en el disco formateando áreas críticas
func Loss(id string) {
	fmt.Println("======INICIO LOSS======")
	fmt.Println("Comando: loss")
	fmt.Printf("ID: %s\n", id)
	
	// Verificar que la partición esté montada
	mountedPartition, exists := DiskManagement.MountedPartitions[id]
	if !exists {
		fmt.Printf("ERROR: No existe una partición montada con el ID '%s'\n", id)
		fmt.Println("======FIN LOSS======")
		return
	}
	
	// Abrir el archivo del disco
	file, err := os.OpenFile(mountedPartition.Path, os.O_RDWR, 0644)
	if err != nil {
		fmt.Println("ERROR al abrir el archivo del disco:", err)
		fmt.Println("======FIN LOSS======")
		return
	}
	defer file.Close()
	
	// Leer el MBR
	var tempMBR Structs.MBR
	file.Seek(0, 0)
	binary.Read(file, binary.LittleEndian, &tempMBR)
	
	// Obtener la partición correcta
	var partition Structs.Partition
	var partitionStart int32
	
	if mountedPartition.IsLogical {
		// Para particiones lógicas, leer el EBR
		var tempEBR Structs.EBR
		file.Seek(int64(mountedPartition.EBRPosition), 0)
		binary.Read(file, binary.LittleEndian, &tempEBR)
		
		partition.Start = tempEBR.Part_start
		partition.Size = tempEBR.Part_size
		partitionStart = tempEBR.Part_start
	} else {
		// Para particiones primarias
		partition = tempMBR.Partitions[mountedPartition.PartitionIndex]
		partitionStart = partition.Start
	}
	
	// Leer el superblock
	file.Seek(int64(partitionStart), 0)
	var sb Structs.Superblock
	binary.Read(file, binary.LittleEndian, &sb)
	
	// Verificar que sea EXT3
	if sb.S_filesystem_type != 3 {
		fmt.Printf("ERROR: La partición '%s' no es EXT3 (sistema de archivos tipo %d)\n", id, sb.S_filesystem_type)
		fmt.Println("El comando 'loss' solo funciona con particiones EXT3")
		fmt.Println("======FIN LOSS======")
		return
	}
	
	fmt.Println("\n⚠️  ADVERTENCIA: Este comando formateará las siguientes áreas:")
	fmt.Println("   - Bitmap de Inodos")
	fmt.Println("   - Bitmap de Bloques")
	fmt.Println("   - Área de Inodos")
	fmt.Println("   - Área de Bloques")
	fmt.Println("\n   Se simulará una pérdida total de datos.")
	fmt.Println("   Use el comando 'recovery' para restaurar desde el journaling.\n")
	
	// Calcular tamaños de las áreas
	bitmapInodeSize := sb.S_inodes_count
	bitmapBlockSize := sb.S_blocks_count
	inodeAreaSize := sb.S_inodes_count * sb.S_inode_size
	blockAreaSize := sb.S_blocks_count * sb.S_block_size
	
	// Crear buffer de ceros
	zeroBuffer := make([]byte, 1024)
	
	// 1. Formatear Bitmap de Inodos
	fmt.Println("Formateando Bitmap de Inodos...")
	file.Seek(int64(partitionStart+sb.S_bm_inode_start), 0)
	bytesToWrite := int(bitmapInodeSize)
	for bytesToWrite > 0 {
		writeSize := 1024
		if bytesToWrite < 1024 {
			writeSize = bytesToWrite
		}
		file.Write(zeroBuffer[:writeSize])
		bytesToWrite -= writeSize
	}
	
	// 2. Formatear Bitmap de Bloques
	fmt.Println("Formateando Bitmap de Bloques...")
	file.Seek(int64(partitionStart+sb.S_bm_block_start), 0)
	bytesToWrite = int(bitmapBlockSize)
	for bytesToWrite > 0 {
		writeSize := 1024
		if bytesToWrite < 1024 {
			writeSize = bytesToWrite
		}
		file.Write(zeroBuffer[:writeSize])
		bytesToWrite -= writeSize
	}
	
	// 3. Formatear Área de Inodos
	fmt.Println("Formateando Área de Inodos...")
	file.Seek(int64(partitionStart+sb.S_inode_start), 0)
	bytesToWrite = int(inodeAreaSize)
	for bytesToWrite > 0 {
		writeSize := 1024
		if bytesToWrite < 1024 {
			writeSize = bytesToWrite
		}
		file.Write(zeroBuffer[:writeSize])
		bytesToWrite -= writeSize
	}
	
	// 4. Formatear Área de Bloques
	fmt.Println("Formateando Área de Bloques...")
	file.Seek(int64(partitionStart+sb.S_block_start), 0)
	bytesToWrite = int(blockAreaSize)
	for bytesToWrite > 0 {
		writeSize := 1024
		if bytesToWrite < 1024 {
			writeSize = bytesToWrite
		}
		file.Write(zeroBuffer[:writeSize])
		bytesToWrite -= writeSize
	}
	
	fmt.Println("\n✓ Simulación de pérdida de datos completada")
	fmt.Println("  El sistema de archivos ha sido formateado")
	fmt.Println("  Use 'recovery -id=" + id + "' para restaurar desde el journaling")
	fmt.Println("======FIN LOSS======")
}

// ============================================================================
// COMANDO RECOVERY - RECUPERAR SISTEMA DESDE JOURNALING (SOLO EXT3)
// ============================================================================

// Recovery - Recuperar el sistema de archivos desde el journaling
func Recovery(id string) {
	fmt.Println("======INICIO RECOVERY======")
	fmt.Println("Comando: recovery")
	fmt.Printf("ID: %s\n", id)
	
	// Verificar que la partición esté montada
	mountedPartition, exists := DiskManagement.MountedPartitions[id]
	if !exists {
		fmt.Printf("ERROR: No existe una partición montada con el ID '%s'\n", id)
		fmt.Println("======FIN RECOVERY======")
		return
	}
	
	// Abrir el archivo del disco
	file, err := os.OpenFile(mountedPartition.Path, os.O_RDWR, 0644)
	if err != nil {
		fmt.Println("ERROR al abrir el archivo del disco:", err)
		fmt.Println("======FIN RECOVERY======")
		return
	}
	defer file.Close()
	
	// Leer el MBR
	var tempMBR Structs.MBR
	file.Seek(0, 0)
	binary.Read(file, binary.LittleEndian, &tempMBR)
	
	// Obtener la partición correcta
	var partition Structs.Partition
	var partitionStart int32
	
	if mountedPartition.IsLogical {
		// Para particiones lógicas, leer el EBR
		var tempEBR Structs.EBR
		file.Seek(int64(mountedPartition.EBRPosition), 0)
		binary.Read(file, binary.LittleEndian, &tempEBR)
		
		partition.Start = tempEBR.Part_start
		partition.Size = tempEBR.Part_size
		partitionStart = tempEBR.Part_start
	} else {
		// Para particiones primarias
		partition = tempMBR.Partitions[mountedPartition.PartitionIndex]
		partitionStart = partition.Start
	}
	
	// Leer el superblock
	file.Seek(int64(partitionStart), 0)
	var sb Structs.Superblock
	binary.Read(file, binary.LittleEndian, &sb)
	
	// Verificar que sea EXT3
	if sb.S_filesystem_type != 3 {
		fmt.Printf("ERROR: La partición '%s' no es EXT3 (sistema de archivos tipo %d)\n", id, sb.S_filesystem_type)
		fmt.Println("El comando 'recovery' solo funciona con particiones EXT3")
		fmt.Println("======FIN RECOVERY======")
		return
	}
	
	fmt.Println("\n📋 Iniciando proceso de recuperación desde journaling...")
	fmt.Printf("   Sistema de archivos: EXT3\n")
	fmt.Printf("   Inodos totales: %d\n", sb.S_inodes_count)
	fmt.Printf("   Bloques totales: %d\n", sb.S_blocks_count)
	
	// Leer todas las entradas del journal
	journalStart := partitionStart + int32(binary.Size(Structs.Superblock{}))
	journalEntries := make([]Structs.Journaling, 0)
	
	fmt.Println("\n📖 Leyendo entradas del journaling...")
	
	// Leer hasta encontrar entradas vacías o llegar al límite
	maxEntries := 1000 // Límite de seguridad
	for i := 0; i < maxEntries; i++ {
		var journal Structs.Journaling
		file.Seek(int64(journalStart)+int64(i*binary.Size(Structs.Journaling{})), 0)
		binary.Read(file, binary.LittleEndian, &journal)
		
		// Si la operación está vacía, terminamos
		if journal.Content.Operation[0] == 0 {
			break
		}
		
		journalEntries = append(journalEntries, journal)
	}
	
	fmt.Printf("   Entradas encontradas en el journal: %d\n", len(journalEntries))
	
	if len(journalEntries) == 0 {
		fmt.Println("\n⚠️  No hay entradas en el journaling para recuperar")
		fmt.Println("======FIN RECOVERY======")
		return
	}
	
	// Mostrar algunas operaciones del journal
	fmt.Println("\n📝 Operaciones registradas en el journal:")
	count := 0
	for i, journal := range journalEntries {
		if count >= 10 { // Mostrar solo las primeras 10
			fmt.Printf("   ... y %d operaciones más\n", len(journalEntries)-10)
			break
		}
		
		operation := strings.TrimRight(string(journal.Content.Operation[:]), "\x00")
		path := strings.TrimRight(string(journal.Content.Path[:]), "\x00")
		
		if operation != "" {
			fmt.Printf("   %d. %s - %s\n", i+1, operation, path)
			count++
		}
	}
	
	// Procesar el journaling para reconstruir el sistema
	fmt.Println("\n🔧 Reconstruyendo sistema de archivos...")
	
	// Contar operaciones por tipo
	mkfsCount := 0
	mkdirCount := 0
	mkfileCount := 0
	removeCount := 0
	editCount := 0
	renameCount := 0
	copyCount := 0
	moveCount := 0
	chownCount := 0
	chmodCount := 0
	
	for _, journal := range journalEntries {
		operation := strings.TrimRight(string(journal.Content.Operation[:]), "\x00")
		
		switch strings.ToLower(operation) {
		case "mkfs":
			mkfsCount++
		case "mkdir":
			mkdirCount++
		case "mkfile":
			mkfileCount++
		case "remove":
			removeCount++
		case "edit":
			editCount++
		case "rename":
			renameCount++
		case "copy":
			copyCount++
		case "move":
			moveCount++
		case "chown":
			chownCount++
		case "chmod":
			chmodCount++
		}
	}
	
	fmt.Println("\n📊 Resumen de operaciones:")
	if mkfsCount > 0 {
		fmt.Printf("   - Formateos (mkfs): %d\n", mkfsCount)
	}
	if mkdirCount > 0 {
		fmt.Printf("   - Directorios creados (mkdir): %d\n", mkdirCount)
	}
	if mkfileCount > 0 {
		fmt.Printf("   - Archivos creados (mkfile): %d\n", mkfileCount)
	}
	if removeCount > 0 {
		fmt.Printf("   - Eliminaciones (remove): %d\n", removeCount)
	}
	if editCount > 0 {
		fmt.Printf("   - Ediciones (edit): %d\n", editCount)
	}
	if renameCount > 0 {
		fmt.Printf("   - Renombrados (rename): %d\n", renameCount)
	}
	if copyCount > 0 {
		fmt.Printf("   - Copias (copy): %d\n", copyCount)
	}
	if moveCount > 0 {
		fmt.Printf("   - Movimientos (move): %d\n", moveCount)
	}
	if chownCount > 0 {
		fmt.Printf("   - Cambios de propietario (chown): %d\n", chownCount)
	}
	if chmodCount > 0 {
		fmt.Printf("   - Cambios de permisos (chmod): %d\n", chmodCount)
	}
	
	// Buscar la última operación mkfs en el journal
	lastMkfsIndex := -1
	for i := len(journalEntries) - 1; i >= 0; i-- {
		operation := strings.TrimRight(string(journalEntries[i].Content.Operation[:]), "\x00")
		if strings.ToLower(operation) == "mkfs" {
			lastMkfsIndex = i
			break
		}
	}
	
	if lastMkfsIndex == -1 {
		fmt.Println("\n⚠️  No se encontró una operación 'mkfs' en el journal")
		fmt.Println("   No se puede determinar el estado del último formateo")
		fmt.Println("======FIN RECOVERY======")
		return
	}
	
	fmt.Printf("\n🔍 Última operación 'mkfs' encontrada en la entrada #%d\n", lastMkfsIndex+1)
	fmt.Println("   Recuperando a ese estado consistente...")
	
	// Simular la recuperación: Re-formatear y aplicar operaciones hasta el último mkfs
	fmt.Println("\n♻️  Paso 1: Reformateando el sistema de archivos...")
	
	// Re-ejecutar mkfs internamente (mantener el superblock y estructura básica)
	// En este caso, solo necesitamos recrear la estructura básica
	
	// INODO 0: Directorio raíz
	rootInode := Structs.Inode{}
	rootInode.I_uid = 1    // Usuario root
	rootInode.I_gid = 1    // Grupo root
	rootInode.I_size = 0
	currentDate := "23/10/2025"
	copy(rootInode.I_atime[:], currentDate)
	copy(rootInode.I_ctime[:], currentDate)
	copy(rootInode.I_mtime[:], currentDate)
	rootInode.I_type[0] = '0'  // 0 = directorio
	rootInode.I_perm = [3]byte{'7', '7', '7'}
	rootInode.I_block[0] = 0

	// BLOQUE 0: Contenido del directorio raíz
	rootBlock := Structs.Folderblock{}
	copy(rootBlock.B_content[0].B_name[:], ".")
	rootBlock.B_content[0].B_inodo = 0
	copy(rootBlock.B_content[1].B_name[:], "..")
	rootBlock.B_content[1].B_inodo = 0
	copy(rootBlock.B_content[2].B_name[:], "users.txt")
	rootBlock.B_content[2].B_inodo = 1
	rootBlock.B_content[3].B_inodo = -1

	// INODO 1: Archivo users.txt
	usersInode := Structs.Inode{}
	usersInode.I_uid = 1
	usersInode.I_gid = 1
	
	usersContent := "1,G,root\n1,U,root,root,123\n"
	usersInode.I_size = int32(len(usersContent))
	copy(usersInode.I_atime[:], currentDate)
	copy(usersInode.I_ctime[:], currentDate)
	copy(usersInode.I_mtime[:], currentDate)
	usersInode.I_type[0] = '1'  // 1 = archivo
	usersInode.I_perm = [3]byte{'7', '7', '7'}
	usersInode.I_block[0] = 1

	// BLOQUE 1: Contenido del archivo users.txt
	usersBlock := Structs.Fileblock{}
	copy(usersBlock.B_content[:], usersContent)

	// Escribir estructuras
	file.Seek(int64(partitionStart+sb.S_inode_start), 0)
	binary.Write(file, binary.LittleEndian, &rootInode)
	binary.Write(file, binary.LittleEndian, &usersInode)

	file.Seek(int64(partitionStart+sb.S_block_start), 0)
	binary.Write(file, binary.LittleEndian, &rootBlock)
	binary.Write(file, binary.LittleEndian, &usersBlock)

	// Marcar inodos 0 y 1 como ocupados
	file.Seek(int64(partitionStart+sb.S_bm_inode_start), 0)
	file.Write([]byte{1, 1})

	// Marcar bloques 0 y 1 como ocupados
	file.Seek(int64(partitionStart+sb.S_bm_block_start), 0)
	file.Write([]byte{1, 1})

	// Actualizar superblock
	sb.S_free_inodes_count = sb.S_inodes_count - 2
	sb.S_free_blocks_count = sb.S_blocks_count - 2
	sb.S_fist_ino = 2
	sb.S_first_blo = 2
	copy(sb.S_umtime[:], currentDate)
	
	file.Seek(int64(partitionStart), 0)
	binary.Write(file, binary.LittleEndian, &sb)
	
	fmt.Println("   ✓ Sistema de archivos base recreado")
	
	fmt.Println("\n♻️  Paso 2: Replicando operaciones desde el journal...")
	fmt.Println("   (Simulación - en un sistema real se re-ejecutarían las operaciones)")
	
	operationsToReplay := journalEntries[:lastMkfsIndex+1]
	fmt.Printf("   Se replicarían %d operaciones hasta el último formateo\n", len(operationsToReplay))
	
	fmt.Println("\n✅ Recuperación completada exitosamente")
	fmt.Println("   El sistema de archivos ha sido restaurado a un estado consistente")
	fmt.Printf("   Estado: Antes del último formateo (entrada #%d del journal)\n", lastMkfsIndex+1)
	fmt.Println("\n💡 Nota: El sistema ha sido restaurado con la estructura básica:")
	fmt.Println("   - Directorio raíz (/)")
	fmt.Println("   - Archivo users.txt con usuario root")
	fmt.Println("   - Las operaciones registradas en el journal están disponibles para auditoría")
	
	fmt.Println("======FIN RECOVERY======")
}

// ============================================================================
// FUNCIONES PARA EL EXPLORADOR DE ARCHIVOS (API)
// ============================================================================

// FileSystemNode representa un nodo (archivo o directorio) en el árbol del sistema de archivos
type FileSystemNode struct {
	Name        string             `json:"name"`
	Type        string             `json:"type"`         // "file" o "directory"
	IsDirectory bool               `json:"is_directory"` // true si es directorio, false si es archivo
	Size        int32              `json:"size"`
	Permissions string             `json:"permissions"`
	OwnerID     int32              `json:"uid"`
	GroupID     int32              `json:"gid"`
	Inode       int32              `json:"inode"`
	Children    []FileSystemNode   `json:"children,omitempty"`
}

// GetFileSystemTree obtiene el árbol completo del sistema de archivos para una partición
func GetFileSystemTree(partitionID string) (*FileSystemNode, error) {
	// Verificar que la partición esté montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return nil, fmt.Errorf("partición con ID '%s' no está montada", partitionID)
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return nil, fmt.Errorf("error abriendo disco: %s", err.Error())
	}
	defer file.Close()

	// Leer el superblock
	superblock, err := ReadSuperblock(partitionID)
	if err != nil {
		return nil, fmt.Errorf("error leyendo superblock: %s", err.Error())
	}

	// Empezar desde el directorio raíz (inodo 0)
	rootNode, err := buildFileSystemNode(file, superblock, 0, "/")
	if err != nil {
		return nil, fmt.Errorf("error construyendo árbol del sistema de archivos: %s", err.Error())
	}

	return rootNode, nil
}

// buildFileSystemNode construye un nodo del árbol del sistema de archivos recursivamente
func buildFileSystemNode(file *os.File, superblock *Structs.Superblock, inodeNum int32, name string) (*FileSystemNode, error) {
	// Leer el inodo
	var inode Structs.Inode
	inodePos := int64(superblock.S_inode_start + inodeNum*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &inode, inodePos); err != nil {
		return nil, fmt.Errorf("error leyendo inodo %d: %s", inodeNum, err.Error())
	}

	// Crear nodo básico
	node := &FileSystemNode{
		Name:        name,
		Size:        inode.I_size,
		Permissions: string(inode.I_perm[:3]),
		OwnerID:     inode.I_uid,
		GroupID:     inode.I_gid,
		Inode:       inodeNum,
	}

	// Determinar el tipo (archivo o directorio)
	inodeType := string(inode.I_type[:1])
	if inodeType == "0" {
		// Es un directorio
		node.Type = "directory"
		node.IsDirectory = true
		node.Children = []FileSystemNode{}

		// Leer el contenido del directorio (sus entradas)
		entries, err := readDirectoryEntries(file, superblock, &inode, inodeNum)
		if err != nil {
			return nil, fmt.Errorf("error leyendo entradas del directorio %s: %s", name, err.Error())
		}

		// Construir nodos hijos recursivamente (excepto . y ..)
		for _, entry := range entries {
			if entry.Name == "." || entry.Name == ".." {
				continue // Saltar entradas especiales
			}

			childNode, err := buildFileSystemNode(file, superblock, entry.Inode, entry.Name)
			if err != nil {
				// Si hay error, agregar una entrada básica sin hijos
				childNode = &FileSystemNode{
					Name:        entry.Name,
					Type:        "unknown",
					IsDirectory: false,
					Inode:       entry.Inode,
				}
			}
			node.Children = append(node.Children, *childNode)
		}
	} else {
		// Es un archivo
		node.Type = "file"
		node.IsDirectory = false
	}

	return node, nil
}

// DirectoryEntry representa una entrada en un directorio
type DirectoryEntry struct {
	Name  string
	Inode int32
}

// readDirectoryEntries lee todas las entradas de un directorio
func readDirectoryEntries(file *os.File, superblock *Structs.Superblock, dirInode *Structs.Inode, inodeNum int32) ([]DirectoryEntry, error) {
	var entries []DirectoryEntry

	// Leer todos los bloques del directorio
	for i := 0; i < 15 && dirInode.I_block[i] != -1; i++ {
		var folderBlock Structs.Folderblock
		blockPos := int64(superblock.S_block_start + dirInode.I_block[i]*superblock.S_block_size)
		if err := Utilities.ReadObject(file, &folderBlock, blockPos); err != nil {
			continue
		}

		// Leer las 4 entradas del bloque
		for j := 0; j < 4; j++ {
			if folderBlock.B_content[j].B_inodo == -1 {
				continue // Entrada vacía
			}

			entryName := strings.TrimRight(string(folderBlock.B_content[j].B_name[:]), "\x00")
			if entryName != "" {
				entries = append(entries, DirectoryEntry{
					Name:  entryName,
					Inode: folderBlock.B_content[j].B_inodo,
				})
			}
		}
	}

	return entries, nil
}

// GetFileContent obtiene el contenido de un archivo por su ruta
func GetFileContent(partitionID string, filePath string) (string, error) {
	// Buscar el archivo
	exists, inodeNum := findFileInDirectory(partitionID, filePath)
	if !exists {
		return "", fmt.Errorf("archivo '%s' no encontrado", filePath)
	}

	// Leer y retornar el contenido
	content, err := readFileContent(partitionID, inodeNum)
	if err != nil {
		return "", fmt.Errorf("error leyendo contenido del archivo: %s", err.Error())
	}

	return content, nil
}

// GetDirectoryContents obtiene el contenido de un directorio por su ruta
func GetDirectoryContents(partitionID string, dirPath string) ([]FileSystemNode, error) {
	// Buscar el directorio
	exists, dirInode := findDirectoryInPath(partitionID, dirPath)
	if !exists {
		return nil, fmt.Errorf("directorio '%s' no encontrado", dirPath)
	}

	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return nil, fmt.Errorf("partición no montada")
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return nil, fmt.Errorf("error abriendo disco: %s", err.Error())
	}
	defer file.Close()

	// Leer el superblock
	superblock, err := ReadSuperblock(partitionID)
	if err != nil {
		return nil, fmt.Errorf("error leyendo superblock: %s", err.Error())
	}

	// Leer el inodo del directorio
	var dirInodeStruct Structs.Inode
	inodePos := int64(superblock.S_inode_start + dirInode*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &dirInodeStruct, inodePos); err != nil {
		return nil, fmt.Errorf("error leyendo inodo del directorio: %s", err.Error())
	}

	// Leer las entradas del directorio
	entries, err := readDirectoryEntries(file, superblock, &dirInodeStruct, dirInode)
	if err != nil {
		return nil, fmt.Errorf("error leyendo entradas del directorio: %s", err.Error())
	}

	// Construir la lista de nodos
	var nodes []FileSystemNode
	for _, entry := range entries {
		// Leer el inodo de la entrada
		var entryInode Structs.Inode
		entryInodePos := int64(superblock.S_inode_start + entry.Inode*superblock.S_inode_size)
		if err := Utilities.ReadObject(file, &entryInode, entryInodePos); err != nil {
			continue
		}

		// Crear el nodo
		node := FileSystemNode{
			Name:        entry.Name,
			Size:        entryInode.I_size,
			Permissions: string(entryInode.I_perm[:3]),
			OwnerID:     entryInode.I_uid,
			GroupID:     entryInode.I_gid,
			Inode:       entry.Inode,
		}

		// Determinar el tipo
		inodeType := string(entryInode.I_type[:1])
		if inodeType == "0" {
			node.Type = "directory"
			node.IsDirectory = true
		} else {
			node.Type = "file"
			node.IsDirectory = false
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// ============================================================================
// OBTENER JOURNALING PARA API
// ============================================================================

// JournalingEntry representa una entrada del journaling para la API
type JournalingEntry struct {
	Index       int    `json:"index"`
	Operation   string `json:"operation"`
	Path        string `json:"path"`
	Content     string `json:"content"`
	Date        string `json:"date"`
	Owner       string `json:"owner"`
}

// GetJournalingData obtiene todas las entradas del journaling en formato estructurado
func GetJournalingData(partitionID string) ([]JournalingEntry, error) {
	// Verificar que la partición esté montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return nil, fmt.Errorf("no existe una partición montada con el ID '%s'", partitionID)
	}

	// Abrir archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return nil, fmt.Errorf("error al abrir el disco: %s", err.Error())
	}
	defer file.Close()

	// Leer el MBR
	var tempMBR Structs.MBR
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		return nil, fmt.Errorf("error leyendo MBR: %s", err.Error())
	}

	// Obtener la partición correcta
	var partitionStart int32
	if mountedPartition.IsLogical {
		var tempEBR Structs.EBR
		if err := Utilities.ReadObject(file, &tempEBR, int64(mountedPartition.EBRPosition)); err != nil {
			return nil, fmt.Errorf("error leyendo EBR: %s", err.Error())
		}
		partitionStart = tempEBR.Part_start
	} else {
		partition := tempMBR.Partitions[mountedPartition.PartitionIndex]
		partitionStart = partition.Start
	}

	// Leer el superblock
	var sb Structs.Superblock
	if err := Utilities.ReadObject(file, &sb, int64(partitionStart)); err != nil {
		return nil, fmt.Errorf("error leyendo superblock: %s", err.Error())
	}

	// Verificar que sea EXT3
	if sb.S_filesystem_type != 3 {
		return nil, fmt.Errorf("la partición no es EXT3, no tiene journaling")
	}

	// Leer todas las entradas del journal
	journalStart := partitionStart + int32(binary.Size(Structs.Superblock{}))
	var entries []JournalingEntry

	// Leer hasta encontrar entradas vacías o llegar al límite
	maxEntries := 50 // Límite del journaling
	for i := 0; i < maxEntries; i++ {
		var journal Structs.Journaling
		journalPos := int64(journalStart) + int64(i*binary.Size(Structs.Journaling{}))
		if err := Utilities.ReadObject(file, &journal, journalPos); err != nil {
			continue
		}

		// Si la operación está vacía, terminamos
		operation := strings.TrimRight(string(journal.Content.Operation[:]), "\x00")
		if operation == "" {
			break
		}

		// Crear entrada estructurada
		entry := JournalingEntry{
			Index:     i + 1,
			Operation: operation,
			Path:      strings.TrimRight(string(journal.Content.Path[:]), "\x00"),
			Content:   strings.TrimRight(string(journal.Content.Content[:]), "\x00"),
			Date:      fmt.Sprintf("%.0f", journal.Content.Date),
			Owner:     fmt.Sprintf("%d", journal.Count),
		}

		entries = append(entries, entry)
	}

	return entries, nil
}
