package DiskManagement

import (
	"proyecto1/Structs"
	"proyecto1/Utilities"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Mapa global para asociar letras de drive con rutas de archivos
var DrivePathMap = make(map[string]string)

// Mapa global para particiones montadas en RAM
var MountedPartitions = make(map[string]Structs.MountedPartition)

// Contadores para generar IDs únicos por disco
var DiskMountCounters = make(map[string]Structs.DiskCounters)

// Lista ordenada de discos para mantener orden cronológico
var DiskOrderList []string

// Función para registrar un drive con su ruta
func RegisterDrive(path string) string {
	// Extraer el nombre del archivo sin extensión para usarlo como drive
	filename := filepath.Base(path)
	driveName := strings.ToUpper(strings.TrimSuffix(filename, filepath.Ext(filename)))
	
	// Si el nombre del archivo tiene más de una letra, usar solo la primera
	if len(driveName) > 1 {
		driveName = string(driveName[0])
	}
	
	DrivePathMap[driveName] = path
	fmt.Println("Drive", driveName, "registrado con ruta:", path)
	return driveName
}

// Función para obtener la ruta de un drive
func GetDrivePath(drive string) (string, bool) {
	path, exists := DrivePathMap[strings.ToUpper(drive)]
	return path, exists
}

func Rmdisk(path string) {
	fmt.Println("======Inicio RMDISK======")
	fmt.Println("Path:", path)

	// Validar que el path no esté vacío
	if path == "" {
		fmt.Println("Error: El parámetro -path es requerido")
		return
	}

	// Verificar que el archivo existe
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("Error: El archivo %s no existe\n", path)
		return
	}

	// Verificar que es un archivo .mia
	if !strings.HasSuffix(strings.ToLower(path), ".mia") {
		fmt.Printf("Error: El archivo %s no es un disco válido (.mia)\n", path)
		return
	}

	// Buscar y remover el drive del mapa si existe
	var driveToRemove string
	for drive, diskPath := range DrivePathMap {
		if diskPath == path {
			driveToRemove = drive
			break
		}
	}

	// Eliminar el archivo
	err := os.Remove(path)
	if err != nil {
		fmt.Printf("Error eliminando el archivo: %v\n", err)
		return
	}

	// Remover del mapa de drives si estaba registrado
	if driveToRemove != "" {
		delete(DrivePathMap, driveToRemove)
		fmt.Printf("Drive %s removido del mapa de drives\n", driveToRemove)
	}

	fmt.Printf("Disco eliminado exitosamente: %s\n", path)
	fmt.Println("======Fin RMDISK======")
}

func Mkdisk(size int, fit string, unit string, path string) {
	fmt.Println("======Inicio MKDISK======")
    fmt.Println("======Parámetros Recibidos======")
	fmt.Println("Size:", size)
	fmt.Println("Fit:", fit, "(default: ff)")
	fmt.Println("Unit:", unit, "(default: m)")
	fmt.Println("Path:", path)

	// validar fit = bf/ff/wf
	if fit != "bf" && fit != "ff" && fit != "wf" {
		fmt.Println("Error: Fit debe ser bf, ff, o wf")
		return
	}

	// validar que el tamaño sea mayor a 0
	if size <= 0 {
		fmt.Println("Error: Tamaño debe ser mayor a 0")
		return
	}

	// validar que unidad sea igual a k o m
	if unit != "k" && unit != "m" {
		fmt.Println("Error: Unidad debe ser k o m")
		return
	}

	// validar que path no esté vacío
	if path == "" {
		fmt.Println("Error: La ruta del archivo es requerida")
		return
	}

	// Verificar y crear directorios si es necesario
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			fmt.Printf("Creando directorio: %s\n", dir)
		}
	}

	// Crear Archivo en la ruta especificada (con directorios automáticos)
	err := Utilities.CreateFile(path)
	if err != nil {
		fmt.Println("Error creando archivo o directorios:", err)
		return
	}

	fmt.Printf("Archivo creado exitosamente en: %s\n", path)

	// Definir el tamaño del archivo
	if unit == "k" {
		size *= 1024
	} else {
		size *= 1024 * 1024
	}

	// Abrir archivo binario
	file, err := Utilities.OpenFile(path)
	if err != nil {
		return
	}

	// Buffer 1024 bytes
	zeroBuffer := make([]byte, 1024)

	// escribir 0 binarios en el archivo
	for i := 0; i < size/1024; i++ {
		err := Utilities.WriteObject(file, zeroBuffer, int64(i*1024))
		if err != nil {
			return
		}
	}

	// crear nueva instancia de MBR
	var newMBR Structs.MBR
	newMBR.MbrSize = int32(size)
	newMBR.Signature = 10                      // random
	copy(newMBR.Fit[:], fit)                   // fit del MRB
	copy(newMBR.CreationDate[:], "2025-08-04") // fecha actual del MBR

	// Escribir MBR al archivo
	if err := Utilities.WriteObject(file, newMBR, 0); err != nil {
		fmt.Println("Error escribiendo MBR al archivo:", err)
		return
	}

	var tempMBR Structs.MBR

	// Leer MBR del archivo para verificar
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		fmt.Println("Error leyendo MBR del archivo:", err)
		return
	}

	// Imprimir MBR para verificar
	fmt.Println("===Data recuperada===")
	fmt.Println("Tamaño del MBR:", tempMBR.MbrSize)
	fmt.Println("Fit:", string(tempMBR.Fit[:]))
	fmt.Println("Fecha de creación:", string(tempMBR.CreationDate[:]))
	fmt.Println("Firma:", tempMBR.Signature)

	// Cerrar el archivo binario
	defer file.Close()

	// Registrar el drive en el mapa
	RegisterDrive(path)

	fmt.Println("======Fin MKDISK======")
}

func Mount(path string, name string) {
	fmt.Println("======Inicio MOUNT======")
	fmt.Println("Path del disco:", path)
	fmt.Println("Nombre de partición:", name)

	// Validar parámetros
	if path == "" {
		fmt.Println("Error: El parámetro -path es requerido")
		return
	}
	if name == "" {
		fmt.Println("Error: El parámetro -name es requerido") 
		return
	}

	// Verificar que el archivo del disco existe
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("Error: El archivo %s no existe\n", path)
		return
	}

	// Verificar que es un archivo .mia
	if !strings.HasSuffix(strings.ToLower(path), ".mia") {
		fmt.Printf("Error: El archivo %s no es un disco válido (.mia)\n", path)
		return
	}

	// Verificar si la partición ya está montada
	for id, mounted := range MountedPartitions {
		if mounted.Path == path && mounted.PartitionName == name {
			fmt.Printf("Error: La partición '%s' del disco '%s' ya está montada con ID '%s'\n", name, path, id)
			return
		}
	}

	// Abrir archivo del disco
	file, err := Utilities.OpenFile(path)
	if err != nil {
		fmt.Println("Error abriendo archivo:", err)
		return
	}
	defer file.Close()

	var tempMBR Structs.MBR
	// Leer MBR del archivo
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		fmt.Println("Error leyendo MBR del archivo:", err)
		return
	}

	// Buscar la partición por nombre
	partitionFound := false
	var mountedPartition Structs.MountedPartition

	// Primero buscar en particiones primarias/extendidas
	for i := 0; i < 4; i++ {
		partitionName := strings.TrimSpace(strings.Trim(string(tempMBR.Partitions[i].Name[:]), "\x00"))
		if partitionName == name && tempMBR.Partitions[i].Size > 0 {
			// Verificar que sea partición primaria (no extendida para montaje)
			partitionType := strings.TrimSpace(strings.Trim(string(tempMBR.Partitions[i].Type[:]), "\x00"))
			if partitionType == "p" {
				// Crear estructura de partición montada
				mountedPartition = Structs.MountedPartition{
					Path:           path,
					PartitionName:  name,
					PartitionIndex: i,
					IsLogical:      false,
				}
				partitionFound = true
				break
			} else {
				fmt.Printf("Error: No se pueden montar particiones extendidas. Solo particiones primarias y lógicas.\n")
				return
			}
		}
	}

	// Si no se encontró en primarias, buscar en lógicas
	if !partitionFound {
		mountedPartition, partitionFound = findLogicalPartition(file, &tempMBR, name, path)
	}

	if !partitionFound {
		fmt.Printf("Error: Partición '%s' no encontrada en el disco '%s'\n", name, path)
		return
	}

	// Generar ID único para la partición
	id := generatePartitionID(path)
	mountedPartition.Id = id

	// Actualizar el estado de la partición en el disco
	if mountedPartition.IsLogical {
		updateLogicalPartitionStatus(file, mountedPartition.EBRPosition, id, true)
	} else {
		updatePrimaryPartitionStatus(file, &tempMBR, mountedPartition.PartitionIndex, id, true)
	}

	// Registrar la partición como montada en RAM
	MountedPartitions[id] = mountedPartition

	fmt.Printf("✓ Partición '%s' montada exitosamente con ID: %s\n", name, id)
	fmt.Printf("  - Tipo: %s\n", map[bool]string{true: "Lógica", false: "Primaria"}[mountedPartition.IsLogical])
	fmt.Printf("  - Disco: %s\n", path)
	fmt.Printf("  - Status: Activa\n")
	fmt.Printf("  - Correlativo: 1\n")
	
	// Mostrar información de cómo se generó el ID
	fmt.Printf("\n--- Información del ID generado ---\n")
	fmt.Printf("  - Carnet: 202201185 → Últimos 2 dígitos: 85\n")
	fmt.Printf("  - Número de partición en disco: %d\n", getPartitionNumberInDisk(path)-1) // -1 porque ya se montó
	fmt.Printf("  - Letra de disco: %c\n", id[3]) // Extraer letra directamente del ID generado
	fmt.Printf("  - ID final: %s\n", id)
	
	// Mostrar particiones montadas actualmente
	showMountedPartitions()

	fmt.Println("======FIN MOUNT======")
}

// Función auxiliar para obtener el número de partición en un disco específico
func getPartitionNumberInDisk(diskPath string) int {
	count := 0
	for _, mounted := range MountedPartitions {
		if mounted.Path == diskPath {
			count++
		}
	}
	return count + 1 // Retorna el siguiente número que se asignaría
}

// Función auxiliar para obtener la letra asignada a un disco
func getDiskLetter(diskPath string) byte {
	// Obtener lista ordenada de discos únicos por orden de primera aparición
	var uniqueDisks []string
	diskFirstSeen := make(map[string]int)
	
	orderCounter := 0
	for _, mounted := range MountedPartitions {
		if _, exists := diskFirstSeen[mounted.Path]; !exists {
			diskFirstSeen[mounted.Path] = orderCounter
			uniqueDisks = append(uniqueDisks, mounted.Path)
			orderCounter++
		}
	}
	
	// Encontrar el índice de este disco
	for i, disk := range uniqueDisks {
		if disk == diskPath {
			return byte('A' + i)
		}
	}
	
	// Si no se encontró, es un disco nuevo - asignar la siguiente letra
	return byte('A' + len(uniqueDisks))
}

// Función para buscar una partición lógica por nombre
func findLogicalPartition(file *os.File, tempMBR *Structs.MBR, name string, path string) (Structs.MountedPartition, bool) {
	// Buscar partición extendida
	var extendedIndex = -1
	for i := 0; i < 4; i++ {
		if string(tempMBR.Partitions[i].Type[:]) == "e" && tempMBR.Partitions[i].Size != 0 {
			extendedIndex = i
			break
		}
	}

	if extendedIndex == -1 {
		return Structs.MountedPartition{}, false // No hay partición extendida
	}

	extendedPartition := tempMBR.Partitions[extendedIndex]
	currentEBRPos := extendedPartition.Start

	// Navegar por los EBRs buscando la partición por nombre
	for {
		var currentEBR Structs.EBR
		if err := Utilities.ReadObject(file, &currentEBR, int64(currentEBRPos)); err != nil {
			break
		}

		// Verificar si es la partición que buscamos
		if currentEBR.Part_size > 0 {
			ebrName := strings.TrimSpace(strings.Trim(string(currentEBR.Part_name[:]), "\x00"))
			if ebrName == name {
				// Crear estructura de partición montada
				mountedPartition := Structs.MountedPartition{
					Path:           path,
					PartitionName:  name,
					PartitionIndex: -1, // No aplica para lógicas
					IsLogical:      true,
					EBRPosition:    currentEBRPos,
				}
				return mountedPartition, true
			}
		}

		// Si no hay siguiente EBR, terminar
		if currentEBR.Part_next == -1 {
			break
		}

		currentEBRPos = currentEBR.Part_next
	}

	return Structs.MountedPartition{}, false
}

// Función para generar ID único basado en el carnet (202201185 -> 85)
func generatePartitionID(diskPath string) string {
	const CARNET_SUFFIX = "85" // Últimos dos dígitos del carnet 202201185

	// Verificar si el disco ya está en la lista ordenada
	diskIndex := -1
	for i, existingDisk := range DiskOrderList {
		if existingDisk == diskPath {
			diskIndex = i
			break
		}
	}
	
	// Si es un disco nuevo, agregarlo al final de la lista
	if diskIndex == -1 {
		DiskOrderList = append(DiskOrderList, diskPath)
		diskIndex = len(DiskOrderList) - 1
	}
	
	// Calcular la letra del disco basada en su posición en la lista
	diskLetter := byte('A' + diskIndex)

	// Contar particiones ya montadas en este disco específico
	partitionCount := 0
	for _, mounted := range MountedPartitions {
		if mounted.Path == diskPath {
			partitionCount++
		}
	}
	
	// El número de partición es el siguiente para este disco
	partitionNumber := partitionCount + 1

	// Generar el ID
	id := fmt.Sprintf("%s%d%c", CARNET_SUFFIX, partitionNumber, diskLetter)

	return id
}

// Función para actualizar el estado de una partición primaria
func updatePrimaryPartitionStatus(file *os.File, tempMBR *Structs.MBR, partitionIndex int, id string, mount bool) error {
	if mount {
		copy(tempMBR.Partitions[partitionIndex].Status[:], "1")     // Activa
		copy(tempMBR.Partitions[partitionIndex].Id[:], id)          // Asignar ID
		tempMBR.Partitions[partitionIndex].Correlative = 1          // Correlativo
	} else {
		copy(tempMBR.Partitions[partitionIndex].Status[:], "0")     // Inactiva
		copy(tempMBR.Partitions[partitionIndex].Id[:], "")          // Limpiar ID
		tempMBR.Partitions[partitionIndex].Correlative = 0          // Limpiar correlativo
	}

	// Escribir MBR actualizado al disco
	return Utilities.WriteObject(file, *tempMBR, 0)
}

// Función para actualizar el estado de una partición lógica
func updateLogicalPartitionStatus(file *os.File, ebrPosition int32, id string, mount bool) error {
	var currentEBR Structs.EBR
	if err := Utilities.ReadObject(file, &currentEBR, int64(ebrPosition)); err != nil {
		return err
	}

	if mount {
		copy(currentEBR.Part_status[:], "1") // Activa
	} else {
		copy(currentEBR.Part_status[:], "0") // Inactiva
	}

	// Escribir EBR actualizado
	return Utilities.WriteObject(file, currentEBR, int64(ebrPosition))
}

// ShowDetailedMountedPartitions - Función pública para mostrar información detallada de particiones montadas
func ShowDetailedMountedPartitions() {
	showDetailedMountedPartitions()
}

// Función para mostrar información detallada de todas las particiones montadas
func showDetailedMountedPartitions() {
	if len(MountedPartitions) == 0 {
		fmt.Println("═══════════════════════════════════════")
		fmt.Println("│        NO HAY PARTICIONES MONTADAS        │")
		fmt.Println("═══════════════════════════════════════")
		fmt.Println("│ No se encontraron particiones montadas    │")
		fmt.Println("│ en el sistema actualmente.                │")
		fmt.Println("│                                           │")
		fmt.Println("│ Use el comando 'mount' para montar        │")
		fmt.Println("│ particiones.                              │")
		fmt.Println("│                                           │")
		fmt.Println("│ Ejemplo:                                  │")
		fmt.Println("│   mount -path=./disco.mia -name=Part1     │")
		fmt.Println("═══════════════════════════════════════")
		return
	}

	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    PARTICIONES MONTADAS EN EL SISTEMA                    ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════╣")
	fmt.Printf("║ Total de particiones montadas: %-30d ║\n", len(MountedPartitions))
	fmt.Printf("║ Carnet del sistema: 202201185 (IDs inician con 85)           ║\n")
	fmt.Println("╠═══════════════════════════════════════════════════════════════╣")

	// Agrupar particiones por disco
	diskGroups := make(map[string][]Structs.MountedPartition)
	for _, partition := range MountedPartitions {
		diskGroups[partition.Path] = append(diskGroups[partition.Path], partition)
	}

	diskCounter := 1
	for diskPath, partitions := range diskGroups {
		fmt.Printf("║                                                              ║\n")
		fmt.Printf("║ DISCO %d: %-51s ║\n", diskCounter, truncateString(diskPath, 51))
		fmt.Printf("║ ├─ Número de particiones montadas: %-25d ║\n", len(partitions))
		
		for i, partition := range partitions {
			typeStr := "Primaria"
			if partition.IsLogical {
				typeStr = "Lógica  "
			}
			
			if i == len(partitions)-1 {
				fmt.Printf("║ └─ [%s] %s - %s - %s ║\n", 
					partition.Id,
					pad(partition.PartitionName, 15),
					typeStr,
					"Activa")
			} else {
				fmt.Printf("║ ├─ [%s] %s - %s - %s ║\n", 
					partition.Id,
					pad(partition.PartitionName, 15),
					typeStr,
					"Activa")
			}
		}
		diskCounter++
	}

	fmt.Println("║                                                              ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════╣")
	fmt.Println("║                        INFORMACIÓN TÉCNICA                           ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════╣")
	
	// Mostrar información de IDs únicos
	uniqueLetters := make(map[byte]bool)
	for _, partition := range MountedPartitions {
		if len(partition.Id) >= 4 {
			letter := partition.Id[3] // Última letra del ID
			uniqueLetters[letter] = true
		}
	}
	
	letterList := ""
	for letter := range uniqueLetters {
		if letterList != "" {
			letterList += ", "
		}
		letterList += string(letter)
	}
	
	fmt.Printf("║ • Letras de disco en uso: %-35s ║\n", letterList)
	fmt.Printf("║ • Montaje en memoria RAM: Sí                                 ║\n")
	fmt.Printf("║ • Estado en disco actualizado: Sí                           ║\n")
	fmt.Printf("║ • IDs únicos generados: %d                                   ║\n", len(MountedPartitions))
	
}

// Función auxiliar para truncar strings largos
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Función auxiliar para hacer padding de strings
func pad(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	padding := length - len(s)
	return s + strings.Repeat(" ", padding)
}

// Función para mostrar todas las particiones montadas
func showMountedPartitions() {
	if len(MountedPartitions) == 0 {
		fmt.Println("No hay particiones montadas actualmente")
		return
	}

	fmt.Println("\n=== PARTICIONES MONTADAS ===")
	for id, partition := range MountedPartitions {
		fmt.Printf("ID: %s | Partición: %s | Tipo: %s | Disco: %s\n", 
			id, 
			partition.PartitionName,
			map[bool]string{true: "Lógica", false: "Primaria"}[partition.IsLogical],
			partition.Path)
	}
	fmt.Println("============================\n")
}

// Función para desmontar una partición (para futura implementación)
func Unmount(id string) {
	fmt.Printf("======INICIO UNMOUNT======\n")
	fmt.Printf("ID: %s\n", id)

	// Buscar la partición montada
	partition, exists := MountedPartitions[id]
	if !exists {
		fmt.Printf("Error: No existe una partición montada con ID '%s'\n", id)
		return
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(partition.Path)
	if err != nil {
		fmt.Println("Error abriendo archivo:", err)
		return
	}
	defer file.Close()

	// Actualizar estado en el disco
	if partition.IsLogical {
		if err := updateLogicalPartitionStatus(file, partition.EBRPosition, "", false); err != nil {
			fmt.Println("Error actualizando estado de partición lógica:", err)
			return
		}
	} else {
		var tempMBR Structs.MBR
		if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
			fmt.Println("Error leyendo MBR:", err)
			return
		}
		
		if err := updatePrimaryPartitionStatus(file, &tempMBR, partition.PartitionIndex, "", false); err != nil {
			fmt.Println("Error actualizando estado de partición primaria:", err)
			return
		}
	}

	// Remover de la lista de particiones montadas
	delete(MountedPartitions, id)

	// Verificar si el disco ya no tiene particiones montadas y limpiarlo de la lista ordenada
	hasPartitionsInDisk := false
	for _, mounted := range MountedPartitions {
		if mounted.Path == partition.Path {
			hasPartitionsInDisk = true
			break
		}
	}
	
	// Si el disco ya no tiene particiones, removerlo de la lista ordenada
	if !hasPartitionsInDisk {
		for i, disk := range DiskOrderList {
			if disk == partition.Path {
				// Remover disco de la lista manteniendo el orden
				DiskOrderList = append(DiskOrderList[:i], DiskOrderList[i+1:]...)
				break
			}
		}
	}

	fmt.Printf("✓ Partición '%s' desmontada exitosamente\n", partition.PartitionName)
	showMountedPartitions()
	fmt.Println("======FIN UNMOUNT======")
}

func Fdisk(size int, path string, name string, type_ string, fit string, unit string) {
	fmt.Println("======INICIO FDISK======")
	fmt.Println("Tamaño: ", size)
	fmt.Println("Path: ", path)
	fmt.Println("Nombre: ", name)
	fmt.Println("Tipo: ", type_, "(default: p)")
	fmt.Println("Fit: ", fit, "(default: wf)")
	fmt.Println("Unit: ", unit, "(default: k)")

	// Validar que el path no esté vacío
	if path == "" {
		fmt.Println("Error: El parámetro -path es requerido")
		return
	}

	// Verificar que el archivo existe
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("Error: El archivo %s no existe. Debe crear el disco primero con mkdisk.\n", path)
		return
	}

	// Verificar que es un archivo .mia
	if !strings.HasSuffix(strings.ToLower(path), ".mia") {
		fmt.Printf("Error: El archivo %s no es un disco válido (.mia)\n", path)
		return
	}

	// Validar que el nombre no esté vacío
	if name == "" {
		fmt.Println("Error: El parámetro -name es requerido")
		return
	}

	// validar fit
	if fit != "bf" && fit != "ff" && fit != "wf" {
		fmt.Println("Error: Fit debe ser bf, ff, o wf")
		return
	}

	// validate tamaño mayor a 0
	if size <= 0 {
		fmt.Println("Error: Tamaño debe ser mayor a 0")
		return
	}

	// validar unidad
	if unit != "b" && unit != "k" && unit != "m" {
		fmt.Println("Error: Unidad debe ser b, k o m")
		return
	}

	// validar tipo de partición
	if type_ != "p" && type_ != "e" && type_ != "l" {
		fmt.Println("Error: Tipo debe ser p (primaria), e (extendida) o l (lógica)")
		return
	}

	// Definir tamaño en bytes
	if unit == "k" {
		size *= 1024
	} else if unit == "m" {
		size *= 1024 * 1024
	}

	// Abrir archivo binario usando la ruta directamente
	file, err := Utilities.OpenFile(path)
	if err != nil {
		fmt.Println("Error abriendo archivo:", err)
		return
	}
	defer file.Close()

	var tempMBR Structs.MBR
	// Leer MBR desde archivo
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		fmt.Println("Error leyendo MBR del archivo :", err)
		return
	}

	// VALIDACIÓN DE ESPACIO DISPONIBLE
	if !validateDiskSpace(&tempMBR, int32(size)) {
		fmt.Println("======FIN FDISK====== (Error de espacio insuficiente)")
		return
	}

	// VALIDACIONES DE TEORÍA DE PARTICIONES
	if type_ == "p" || type_ == "e" {
		// Para particiones primarias y extendidas
		if !validatePrimaryExtendedPartition(&tempMBR, type_, name) {
			fmt.Println("======FIN FDISK====== (Error de validación)")
			return
		}
		createPrimaryOrExtended(file, &tempMBR, size, name, type_, fit)
	} else if type_ == "l" {
		// Para particiones lógicas
		if !validateLogicalPartition(&tempMBR, name) {
			fmt.Println("======FIN FDISK====== (Error de validación)")
			return
		}
		createLogicalPartition(file, &tempMBR, size, name, fit)
	}

	fmt.Println("======FIN FDISK======")
}

// Función para validar particiones primarias y extendidas
func validatePrimaryExtendedPartition(tempMBR *Structs.MBR, type_ string, name string) bool {
	// Contar particiones primarias y extendidas ocupadas
	primaryExtendedCount := 0
	extendedExists := false
	
	for i := 0; i < 4; i++ {
		if tempMBR.Partitions[i].Size != 0 {
			primaryExtendedCount++
			
			// Verificar si ya existe una partición extendida
			partitionType := strings.TrimSpace(strings.Trim(string(tempMBR.Partitions[i].Type[:]), "\x00"))
			if partitionType == "e" {
				extendedExists = true
			}
			
			// Verificar que no exista una partición con el mismo nombre
			existingName := strings.TrimSpace(strings.Trim(string(tempMBR.Partitions[i].Name[:]), "\x00"))
			if existingName == name {
				fmt.Printf("Error: Ya existe una partición con el nombre '%s'\n", name)
				return false
			}
		}
	}
	
	// RESTRICCIÓN 1: La suma de primarias y extendidas debe ser como máximo 4
	if primaryExtendedCount >= 4 {
		fmt.Println("Error: No se pueden crear más particiones. Máximo 4 particiones primarias/extendidas permitidas")
		return false
	}
	
	// RESTRICCIÓN 2: Solo puede haber una partición extendida por disco
	if type_ == "e" && extendedExists {
		fmt.Println("Error: Solo puede haber una partición extendida por disco")
		return false
	}
	
	return true
}

// Función para validar particiones lógicas
func validateLogicalPartition(tempMBR *Structs.MBR, name string) bool {
	// RESTRICCIÓN 3: No se puede crear una partición lógica si no hay una extendida
	extendedIndex := -1
	for i := 0; i < 4; i++ {
		if tempMBR.Partitions[i].Size != 0 && string(tempMBR.Partitions[i].Type[:]) == "e" {
			extendedIndex = i
			break
		}
	}
	
	if extendedIndex == -1 {
		fmt.Println("Error: No se puede crear una partición lógica sin una partición extendida")
		return false
	}
	
	// RESTRICCIÓN 4: Verificar que no exista una partición lógica con el mismo nombre
	return !logicalPartitionNameExists(tempMBR, name, extendedIndex)
}

// Función para verificar si ya existe una partición lógica con el mismo nombre
func logicalPartitionNameExists(tempMBR *Structs.MBR, name string, extendedIndex int) bool {
	// Por ahora retornamos false, pero la validación completa se hará en createLogicalPartition
	// donde tenemos acceso al archivo para navegar por los EBRs
	return false
}

// Función para validar que hay espacio suficiente en el disco
func validateDiskSpace(tempMBR *Structs.MBR, newPartitionSize int32) bool {
	// Calcular el espacio total usado por las particiones existentes
	var totalUsedSpace int32 = 0
	
	// Sumar el tamaño de todas las particiones primarias y extendidas
	for i := 0; i < 4; i++ {
		if tempMBR.Partitions[i].Size != 0 {
			totalUsedSpace += tempMBR.Partitions[i].Size
		}
	}
	
	// El tamaño del disco incluye el espacio para el MBR (164 bytes iniciales)
	// El espacio disponible para particiones es el tamaño total menos el MBR
	availableSpace := tempMBR.MbrSize - 164 // 164 bytes para el MBR
	
	// Verificar si hay espacio suficiente para la nueva partición
	if totalUsedSpace + newPartitionSize > availableSpace {
		fmt.Printf("Error: Espacio insuficiente en el disco\n")
		fmt.Printf("  - Tamaño total del disco: %d bytes (%.2f MB)\n", tempMBR.MbrSize, float64(tempMBR.MbrSize)/(1024*1024))
		fmt.Printf("  - Espacio disponible para particiones: %d bytes (%.2f MB)\n", availableSpace, float64(availableSpace)/(1024*1024))
		fmt.Printf("  - Espacio usado actualmente: %d bytes (%.2f MB)\n", totalUsedSpace, float64(totalUsedSpace)/(1024*1024))
		fmt.Printf("  - Espacio requerido para nueva partición: %d bytes (%.2f MB)\n", newPartitionSize, float64(newPartitionSize)/(1024*1024))
		fmt.Printf("  - Espacio libre restante: %d bytes (%.2f MB)\n", availableSpace-totalUsedSpace, float64(availableSpace-totalUsedSpace)/(1024*1024))
		return false
	}
	
	return true
}

func createPrimaryOrExtended(file *os.File, tempMBR *Structs.MBR, size int, name string, type_ string, fit string) {
	var gap = int32(0)
	// Iterar por las particiones para calcular espacios
	for i := 0; i < 4; i++ {
		if tempMBR.Partitions[i].Size != 0 {
			gap = tempMBR.Partitions[i].Start + tempMBR.Partitions[i].Size
		}
	}

	// Buscar partición vacía en el MBR
	var foundEmpty bool
	for i := 0; i < 4; i++ {
		if tempMBR.Partitions[i].Size == 0 {
			foundEmpty = true
			// Crear nueva partición
			tempMBR.Partitions[i].Size = int32(size)   // Set size
			copy(tempMBR.Partitions[i].Name[:], name)  // Set name
			copy(tempMBR.Partitions[i].Fit[:], fit)    // Set fit
			copy(tempMBR.Partitions[i].Status[:], "0") // Set status = 0 (inactiva)
			copy(tempMBR.Partitions[i].Type[:], type_) // Set type

			if gap > 0 {
				tempMBR.Partitions[i].Start = gap
			} else {
				tempMBR.Partitions[i].Start = int32(binary.Size(*tempMBR))
			}

			// Si es partición extendida, inicializar el primer EBR
			if type_ == "e" {
				initializeExtendedPartition(file, tempMBR.Partitions[i].Start)
			}
			break
		}
	}

	if !foundEmpty {
		fmt.Println("Error: No se encontró partición vacía en el MBR")
		return
	}

	// Sobreescribir MBR en el archivo
	if err := Utilities.WriteObject(file, *tempMBR, 0); err != nil {
		fmt.Println("Error escribiendo MBR en el archivo:", err)
		return
	}

	fmt.Println("Partición", type_, "creada exitosamente")
}

func initializeExtendedPartition(file *os.File, start int32) {
	// Crear EBR vacío al inicio de la partición extendida
	var emptyEBR Structs.EBR
	copy(emptyEBR.Part_status[:], "0")
	copy(emptyEBR.Part_fit[:], "f")
	emptyEBR.Part_start = -1
	emptyEBR.Part_size = 0
	emptyEBR.Part_next = -1
	copy(emptyEBR.Part_name[:], "")

	// Escribir EBR vacío al inicio de la partición extendida
	if err := Utilities.WriteObject(file, emptyEBR, int64(start)); err != nil {
		fmt.Println("Error inicializando partición extendida:", err)
	}
}

func createLogicalPartition(file *os.File, tempMBR *Structs.MBR, size int, name string, fit string) {
	// Buscar partición extendida
	var extendedIndex = -1
	for i := 0; i < 4; i++ {
		if string(tempMBR.Partitions[i].Type[:]) == "e" && tempMBR.Partitions[i].Size != 0 {
			extendedIndex = i
			break
		}
	}

	if extendedIndex == -1 {
		fmt.Println("Error: No existe partición extendida para crear partición lógica")
		return
	}

	extendedPartition := tempMBR.Partitions[extendedIndex]

	// Verificar que no exista una partición lógica con el mismo nombre
	if checkLogicalPartitionNameExists(file, extendedPartition, name) {
		fmt.Printf("Error: Ya existe una partición lógica con el nombre '%s'\n", name)
		return
	}

	// Navegar por la lista de EBRs para encontrar espacio
	currentEBRPos := extendedPartition.Start
	var lastEBRPos int32 = -1
	var newEBRPos int32

	for {
		var currentEBR Structs.EBR
		if err := Utilities.ReadObject(file, &currentEBR, int64(currentEBRPos)); err != nil {
			fmt.Println("Error leyendo EBR:", err)
			return
		}

		// Si es el primer EBR y está vacío
		if currentEBR.Part_size == 0 {
			newEBRPos = currentEBRPos
			break
		}

		// Si hay siguiente EBR, continuar
		if currentEBR.Part_next != -1 {
			lastEBRPos = currentEBRPos
			currentEBRPos = currentEBR.Part_next
		} else {
			// No hay más EBRs, crear uno nuevo al final
			newEBRPos = currentEBR.Part_start + currentEBR.Part_size
			lastEBRPos = currentEBRPos
			break
		}
	}

	// Verificar que hay espacio suficiente en la partición extendida
	extendedEnd := extendedPartition.Start + extendedPartition.Size
	if newEBRPos + int32(binary.Size(Structs.EBR{})) + int32(size) > extendedEnd {
		fmt.Println("Error: No hay espacio suficiente en la partición extendida")
		return
	}

	// Crear nuevo EBR
	var newEBR Structs.EBR
	copy(newEBR.Part_status[:], "0") // Inactiva inicialmente
	copy(newEBR.Part_fit[:], fit)
	newEBR.Part_start = newEBRPos + int32(binary.Size(Structs.EBR{})) // Datos después del EBR
	newEBR.Part_size = int32(size)
	newEBR.Part_next = -1 // Es el último por ahora
	copy(newEBR.Part_name[:], name)

	// Escribir el nuevo EBR
	if err := Utilities.WriteObject(file, newEBR, int64(newEBRPos)); err != nil {
		fmt.Println("Error escribiendo nuevo EBR:", err)
		return
	}

	// Si no es el primer EBR, actualizar el anterior para que apunte al nuevo
	if lastEBRPos != -1 && lastEBRPos != newEBRPos {
		var lastEBR Structs.EBR
		if err := Utilities.ReadObject(file, &lastEBR, int64(lastEBRPos)); err != nil {
			fmt.Println("Error leyendo EBR anterior:", err)
			return
		}
		
		lastEBR.Part_next = newEBRPos
		if err := Utilities.WriteObject(file, lastEBR, int64(lastEBRPos)); err != nil {
			fmt.Println("Error actualizando EBR anterior:", err)
			return
		}
	}

	fmt.Println("Partición lógica creada exitosamente")
}

func listLogicalPartitions(file *os.File, extendedPartition Structs.Partition) {
	fmt.Println("=== Particiones Lógicas ===")
	currentEBRPos := extendedPartition.Start

	for {
		var currentEBR Structs.EBR
		if err := Utilities.ReadObject(file, &currentEBR, int64(currentEBRPos)); err != nil {
			fmt.Println("Error leyendo EBR:", err)
			break
		}

		// Si el EBR tiene datos válidos, contarlo
		if currentEBR.Part_size > 0 {
			fmt.Printf("Partición lógica encontrada: %s\n", string(currentEBR.Part_name[:]))
		}

		// Si no hay siguiente EBR, terminar
		if currentEBR.Part_next == -1 {
			break
		}

		currentEBRPos = currentEBR.Part_next
	}
}

// FdiskAdd - Agregar o quitar espacio de una partición
func FdiskAdd(path string, name string, add int, unit string) {
	fmt.Println("======INICIO FDISK ADD======")
	fmt.Println("Path:", path)
	fmt.Println("Nombre:", name)
	fmt.Println("Add:", add)
	fmt.Println("Unit:", unit)

	// Validar parámetros
	if path == "" {
		fmt.Println("Error: El parámetro -path es requerido")
		return
	}
	if name == "" {
		fmt.Println("Error: El parámetro -name es requerido")
		return
	}
	if add == 0 {
		fmt.Println("Error: El parámetro -add no puede ser 0")
		return
	}

	// Validar unidad
	if unit != "b" && unit != "k" && unit != "m" {
		fmt.Println("Error: Unidad debe ser b, k o m")
		return
	}

	// Verificar que el archivo existe
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("Error: El archivo %s no existe\n", path)
		return
	}

	// Convertir a bytes
	sizeInBytes := add
	if unit == "k" {
		sizeInBytes *= 1024
	} else if unit == "m" {
		sizeInBytes *= 1024 * 1024
	}

	// Abrir archivo
	file, err := Utilities.OpenFile(path)
	if err != nil {
		fmt.Println("Error abriendo archivo:", err)
		return
	}
	defer file.Close()

	var tempMBR Structs.MBR
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		fmt.Println("Error leyendo MBR:", err)
		return
	}

	// Buscar la partición
	partitionFound := false
	partitionIndex := -1

	// Buscar en particiones primarias/extendidas
	for i := 0; i < 4; i++ {
		partitionName := strings.TrimSpace(strings.Trim(string(tempMBR.Partitions[i].Name[:]), "\x00"))
		if partitionName == name && tempMBR.Partitions[i].Size > 0 {
			partitionIndex = i
			partitionFound = true
			break
		}
	}

	if !partitionFound {
		// Buscar en particiones lógicas
		if modifyLogicalPartitionSize(file, &tempMBR, name, int32(sizeInBytes)) {
			fmt.Println("Partición lógica modificada exitosamente")
			fmt.Println("======FIN FDISK ADD======")
			return
		}
		fmt.Printf("Error: Partición '%s' no encontrada\n", name)
		return
	}

	// Modificar partición primaria/extendida
	partition := &tempMBR.Partitions[partitionIndex]
	newSize := int32(int(partition.Size) + sizeInBytes)

	// Validar que el nuevo tamaño sea positivo
	if newSize <= 0 {
		fmt.Println("Error: El nuevo tamaño resultaría en una partición negativa o vacía")
		fmt.Printf("Tamaño actual: %d bytes, intentando %s: %d bytes\n", 
			partition.Size, 
			map[bool]string{true: "agregar", false: "quitar"}[add > 0],
			abs(sizeInBytes))
		return
	}

	// Si se está agregando espacio, verificar que hay espacio disponible después de la partición
	if add > 0 {
		// Calcular el fin de la partición actual
		partitionEnd := partition.Start + partition.Size
		
		// Buscar si hay otra partición inmediatamente después
		nextPartitionStart := tempMBR.MbrSize // Por defecto, el fin del disco
		for i := 0; i < 4; i++ {
			if i != partitionIndex && tempMBR.Partitions[i].Size > 0 {
				if tempMBR.Partitions[i].Start > partitionEnd && tempMBR.Partitions[i].Start < nextPartitionStart {
					nextPartitionStart = tempMBR.Partitions[i].Start
				}
			}
		}
		
		// Verificar que hay espacio suficiente
		availableSpace := nextPartitionStart - partitionEnd
		if int32(sizeInBytes) > availableSpace {
			fmt.Printf("Error: No hay espacio suficiente después de la partición\n")
			fmt.Printf("Espacio disponible: %d bytes (%.2f MB)\n", availableSpace, float64(availableSpace)/(1024*1024))
			fmt.Printf("Espacio requerido: %d bytes (%.2f MB)\n", sizeInBytes, float64(sizeInBytes)/(1024*1024))
			return
		}
	}

	// Aplicar el cambio
	partition.Size = newSize

	// Escribir MBR actualizado
	if err := Utilities.WriteObject(file, tempMBR, 0); err != nil {
		fmt.Println("Error escribiendo MBR actualizado:", err)
		return
	}

	fmt.Printf("✓ Partición '%s' modificada exitosamente\n", name)
	fmt.Printf("  Tamaño anterior: %d bytes (%.2f MB)\n", partition.Size - int32(sizeInBytes), float64(partition.Size - int32(sizeInBytes))/(1024*1024))
	fmt.Printf("  Tamaño nuevo: %d bytes (%.2f MB)\n", partition.Size, float64(partition.Size)/(1024*1024))
	fmt.Printf("  Cambio: %+d bytes (%+.2f MB)\n", sizeInBytes, float64(sizeInBytes)/(1024*1024))
	fmt.Println("======FIN FDISK ADD======")
}

// FdiskDelete - Eliminar una partición
func FdiskDelete(path string, name string, deleteType string) {
	fmt.Println("======INICIO FDISK DELETE======")
	fmt.Println("Path:", path)
	fmt.Println("Nombre:", name)
	fmt.Println("Tipo de eliminación:", deleteType)

	// Validar parámetros
	if path == "" {
		fmt.Println("Error: El parámetro -path es requerido")
		return
	}
	if name == "" {
		fmt.Println("Error: El parámetro -name es requerido")
		return
	}
	if deleteType != "fast" && deleteType != "full" {
		fmt.Println("Error: El parámetro -delete debe ser 'fast' o 'full'")
		return
	}

	// Verificar que el archivo existe
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("Error: El archivo %s no existe\n", path)
		return
	}

	// Advertencia (sin solicitar confirmación para evitar bloqueo en API)
	fmt.Printf("\n⚠️  ADVERTENCIA: Eliminando la partición '%s'\n", name)
	fmt.Printf("Tipo de eliminación: %s\n", deleteType)
	if deleteType == "full" {
		fmt.Println("Esta operación sobrescribirá los datos con \\0 (puede tardar)")
	}

	// Abrir archivo
	file, err := Utilities.OpenFile(path)
	if err != nil {
		fmt.Println("Error abriendo archivo:", err)
		return
	}
	defer file.Close()

	var tempMBR Structs.MBR
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		fmt.Println("Error leyendo MBR:", err)
		return
	}

	// Buscar la partición
	partitionFound := false
	partitionIndex := -1
	isExtended := false

	for i := 0; i < 4; i++ {
		partitionName := strings.TrimSpace(strings.Trim(string(tempMBR.Partitions[i].Name[:]), "\x00"))
		if partitionName == name && tempMBR.Partitions[i].Size > 0 {
			partitionIndex = i
			partitionFound = true
			partitionType := strings.TrimSpace(strings.Trim(string(tempMBR.Partitions[i].Type[:]), "\x00"))
			if partitionType == "e" {
				isExtended = true
			}
			break
		}
	}

	if !partitionFound {
		// Buscar en particiones lógicas
		if deleteLogicalPartition(file, &tempMBR, name, deleteType) {
			fmt.Println("Partición lógica eliminada exitosamente")
			fmt.Println("======FIN FDISK DELETE======")
			return
		}
		fmt.Printf("Error: Partición '%s' no encontrada\n", name)
		return
	}

	// Si es extendida, eliminar todas las lógicas primero
	if isExtended {
		fmt.Println("Eliminando particiones lógicas dentro de la partición extendida...")
		deleteAllLogicalPartitions(file, &tempMBR, tempMBR.Partitions[partitionIndex], deleteType)
	}

	// Guardar información de la partición antes de eliminar
	partition := tempMBR.Partitions[partitionIndex]

	// Si es eliminación completa (full), llenar con ceros
	if deleteType == "full" {
		fmt.Println("Sobrescribiendo datos de la partición con \\0...")
		zeroBuffer := make([]byte, 1024)
		bytesToWrite := int(partition.Size)
		offset := int64(partition.Start)
		
		for bytesToWrite > 0 {
			writeSize := 1024
			if bytesToWrite < 1024 {
				writeSize = bytesToWrite
			}
			if err := Utilities.WriteObject(file, zeroBuffer[:writeSize], offset); err != nil {
				fmt.Println("Error sobrescribiendo datos:", err)
				return
			}
			bytesToWrite -= writeSize
			offset += int64(writeSize)
		}
		fmt.Println("Datos sobrescritos exitosamente")
	}

	// Marcar la partición como vacía en el MBR
	tempMBR.Partitions[partitionIndex] = Structs.Partition{
		Status:      [1]byte{},
		Type:        [1]byte{},
		Fit:         [2]byte{},
		Start:       0,
		Size:        0,
		Name:        [16]byte{},
		Correlative: 0,
		Id:          [4]byte{},
	}

	// Escribir MBR actualizado
	if err := Utilities.WriteObject(file, tempMBR, 0); err != nil {
		fmt.Println("Error escribiendo MBR actualizado:", err)
		return
	}

	fmt.Printf("✓ Partición '%s' eliminada exitosamente\n", name)
	fmt.Printf("  Tipo: %s\n", string(partition.Type[:]))
	fmt.Printf("  Tamaño liberado: %d bytes (%.2f MB)\n", partition.Size, float64(partition.Size)/(1024*1024))
	fmt.Println("======FIN FDISK DELETE======")
}

// Función auxiliar para valor absoluto
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Función auxiliar para modificar el tamaño de una partición lógica
func modifyLogicalPartitionSize(file *os.File, tempMBR *Structs.MBR, name string, sizeChange int32) bool {
	// Buscar partición extendida
	var extendedIndex = -1
	for i := 0; i < 4; i++ {
		if string(tempMBR.Partitions[i].Type[:]) == "e" && tempMBR.Partitions[i].Size != 0 {
			extendedIndex = i
			break
		}
	}

	if extendedIndex == -1 {
		return false
	}

	extendedPartition := tempMBR.Partitions[extendedIndex]
	currentEBRPos := extendedPartition.Start

	// Navegar por los EBRs
	for {
		var currentEBR Structs.EBR
		if err := Utilities.ReadObject(file, &currentEBR, int64(currentEBRPos)); err != nil {
			break
		}

		if currentEBR.Part_size > 0 {
			ebrName := strings.TrimSpace(strings.Trim(string(currentEBR.Part_name[:]), "\x00"))
			if ebrName == name {
				newSize := currentEBR.Part_size + sizeChange

				// Validar que el nuevo tamaño sea positivo
				if newSize <= 0 {
					fmt.Println("Error: El nuevo tamaño resultaría en una partición negativa o vacía")
					return false
				}

				// Si se está agregando espacio, verificar disponibilidad
				if sizeChange > 0 {
					ebrEnd := currentEBR.Part_start + currentEBR.Part_size
					nextEBRStart := extendedPartition.Start + extendedPartition.Size
					
					if currentEBR.Part_next != -1 {
						nextEBRStart = currentEBR.Part_next
					}
					
					availableSpace := nextEBRStart - ebrEnd
					if sizeChange > availableSpace {
						fmt.Printf("Error: No hay espacio suficiente después de la partición lógica\n")
						fmt.Printf("Espacio disponible: %d bytes\n", availableSpace)
						return false
					}
				}

				// Aplicar cambio
				currentEBR.Part_size = newSize
				if err := Utilities.WriteObject(file, currentEBR, int64(currentEBRPos)); err != nil {
					fmt.Println("Error actualizando EBR:", err)
					return false
				}

				return true
			}
		}

		if currentEBR.Part_next == -1 {
			break
		}
		currentEBRPos = currentEBR.Part_next
	}

	return false
}

// Función auxiliar para eliminar una partición lógica
func deleteLogicalPartition(file *os.File, tempMBR *Structs.MBR, name string, deleteType string) bool {
	// Buscar partición extendida
	var extendedIndex = -1
	for i := 0; i < 4; i++ {
		if string(tempMBR.Partitions[i].Type[:]) == "e" && tempMBR.Partitions[i].Size != 0 {
			extendedIndex = i
			break
		}
	}

	if extendedIndex == -1 {
		return false
	}

	extendedPartition := tempMBR.Partitions[extendedIndex]
	currentEBRPos := extendedPartition.Start
	var prevEBRPos int32 = -1

	// Navegar por los EBRs
	for {
		var currentEBR Structs.EBR
		if err := Utilities.ReadObject(file, &currentEBR, int64(currentEBRPos)); err != nil {
			break
		}

		if currentEBR.Part_size > 0 {
			ebrName := strings.TrimSpace(strings.Trim(string(currentEBR.Part_name[:]), "\x00"))
			if ebrName == name {
				// Si es eliminación completa, llenar con ceros
				if deleteType == "full" {
					fmt.Println("Sobrescribiendo datos de la partición lógica con \\0...")
					zeroBuffer := make([]byte, 1024)
					bytesToWrite := int(currentEBR.Part_size)
					offset := int64(currentEBR.Part_start)
					
					for bytesToWrite > 0 {
						writeSize := 1024
						if bytesToWrite < 1024 {
							writeSize = bytesToWrite
						}
						Utilities.WriteObject(file, zeroBuffer[:writeSize], offset)
						bytesToWrite -= writeSize
						offset += int64(writeSize)
					}
				}

				// Actualizar la lista enlazada de EBRs
				if prevEBRPos != -1 {
					// Hay un EBR anterior, actualizar su Part_next
					var prevEBR Structs.EBR
					Utilities.ReadObject(file, &prevEBR, int64(prevEBRPos))
					prevEBR.Part_next = currentEBR.Part_next
					Utilities.WriteObject(file, prevEBR, int64(prevEBRPos))
				}

				// Marcar el EBR actual como vacío
				var emptyEBR Structs.EBR
				copy(emptyEBR.Part_status[:], "0")
				emptyEBR.Part_start = -1
				emptyEBR.Part_size = 0
				emptyEBR.Part_next = -1
				Utilities.WriteObject(file, emptyEBR, int64(currentEBRPos))

				return true
			}
		}

		if currentEBR.Part_next == -1 {
			break
		}
		prevEBRPos = currentEBRPos
		currentEBRPos = currentEBR.Part_next
	}

	return false
}

// Función auxiliar para eliminar todas las particiones lógicas
func deleteAllLogicalPartitions(file *os.File, tempMBR *Structs.MBR, extendedPartition Structs.Partition, deleteType string) {
	currentEBRPos := extendedPartition.Start

	for {
		var currentEBR Structs.EBR
		if err := Utilities.ReadObject(file, &currentEBR, int64(currentEBRPos)); err != nil {
			break
		}

		if currentEBR.Part_size > 0 {
			ebrName := strings.TrimSpace(strings.Trim(string(currentEBR.Part_name[:]), "\x00"))
			fmt.Printf("  Eliminando partición lógica: %s\n", ebrName)

			// Si es eliminación completa, llenar con ceros
			if deleteType == "full" {
				zeroBuffer := make([]byte, 1024)
				bytesToWrite := int(currentEBR.Part_size)
				offset := int64(currentEBR.Part_start)
				
				for bytesToWrite > 0 {
					writeSize := 1024
					if bytesToWrite < 1024 {
						writeSize = bytesToWrite
					}
					Utilities.WriteObject(file, zeroBuffer[:writeSize], offset)
					bytesToWrite -= writeSize
					offset += int64(writeSize)
				}
			}
		}

		if currentEBR.Part_next == -1 {
			break
		}
		currentEBRPos = currentEBR.Part_next
	}
}

func Rep(name string, path string, id string, drive string) {
	fmt.Println("======INICIO REP======")
	fmt.Println("Nombre:", name)
	fmt.Println("Path:", path)
	fmt.Println("Id:", id)
	fmt.Println("Drive:", drive)

	if name == "mbr" && drive != "" {
		reportMBR(drive)
	} else if name == "disk" && drive != "" {
		reportDisk(drive)
	} else {
		fmt.Println("Error: Parámetros inválidos para el reporte")
	}

	fmt.Println("======FIN REP======")
}

func reportMBR(drive string) {
	fmt.Println("=== REPORTE MBR ===")
	
	// Abrir archivo binario usando el mapa de drives
	filepath, exists := GetDrivePath(drive)
	if !exists {
		fmt.Printf("Error: Drive %s no encontrado. Asegúrate de haber creado el disco primero con mkdisk.\n", drive)
		return
	}
	
	file, err := Utilities.OpenFile(filepath)
	if err != nil {
		fmt.Println("Error abriendo archivo:", err)
		return
	}
	defer file.Close()

	var tempMBR Structs.MBR
	// Leer MBR del archivo
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		fmt.Println("Error leyendo MBR del archivo", err)
		return
	}

	// Mostrar información del MBR
	fmt.Printf("MBR - Tamaño: %d, Fecha: %s, Fit: %s\n", tempMBR.MbrSize, string(tempMBR.CreationDate[:]), string(tempMBR.Fit[:]))

	// Mostrar particiones lógicas si existen
	for i := 0; i < 4; i++ {
		if string(tempMBR.Partitions[i].Type[:]) == "e" && tempMBR.Partitions[i].Size != 0 {
			fmt.Println("\n=== PARTICIONES LÓGICAS EN PARTICIÓN EXTENDIDA ===")
			listLogicalPartitions(file, tempMBR.Partitions[i])
		}
	}
}

func reportDisk(drive string) {
	fmt.Println("=== REPORTE DISCO ===")
	
	// Abrir archivo binario usando el mapa de drives
	filepath, exists := GetDrivePath(drive)
	if !exists {
		fmt.Printf("Error: Drive %s no encontrado. Asegúrate de haber creado el disco primero con mkdisk.\n", drive)
		return
	}
	
	file, err := Utilities.OpenFile(filepath)
	if err != nil {
		fmt.Println("Error abriendo archivo:", err)
		return
	}
	defer file.Close()

	var tempMBR Structs.MBR
	// Leer MBR del archivo
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		fmt.Println("Error leyendo MBR del archivo", err)
		return
	}

	fmt.Printf("Tamaño total del disco: %d bytes\n", tempMBR.MbrSize)
	fmt.Printf("Fecha de creación: %s\n", string(tempMBR.CreationDate[:]))
	fmt.Printf("Fit: %s\n", string(tempMBR.Fit[:]))
	
	usedSpace := int32(binary.Size(tempMBR))
	
	fmt.Println("\n=== DISTRIBUCIÓN DEL ESPACIO ===")
	for i := 0; i < 4; i++ {
		if tempMBR.Partitions[i].Size != 0 {
			fmt.Printf("Partición %d: %s (%s) - %d bytes\n", 
				i+1, 
				string(tempMBR.Partitions[i].Name[:]), 
				string(tempMBR.Partitions[i].Type[:]),
				tempMBR.Partitions[i].Size)
			usedSpace += tempMBR.Partitions[i].Size
		}
	}
	
	freeSpace := tempMBR.MbrSize - usedSpace
	fmt.Printf("Espacio libre: %d bytes\n", freeSpace)
}

// Función para verificar si ya existe una partición lógica con el nombre dado
func checkLogicalPartitionNameExists(file *os.File, extendedPartition Structs.Partition, name string) bool {
	currentEBRPos := extendedPartition.Start

	for {
		var currentEBR Structs.EBR
		if err := Utilities.ReadObject(file, &currentEBR, int64(currentEBRPos)); err != nil {
			break // Error leyendo EBR, salir del bucle
		}

		// Si el EBR tiene datos válidos, verificar el nombre
		if currentEBR.Part_size > 0 {
			existingName := strings.TrimSpace(strings.Trim(string(currentEBR.Part_name[:]), "\x00"))
			if existingName == name {
				return true // Se encontró una partición con el mismo nombre
			}
		}

		// Si no hay siguiente EBR, terminar
		if currentEBR.Part_next == -1 {
			break
		}

		currentEBRPos = currentEBR.Part_next
	}

	return false // No se encontró ninguna partición con el mismo nombre
}