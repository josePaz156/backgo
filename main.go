package main

import (
	"proyecto1/Analyzer"
	"proyecto1/FileSystem"
	"proyecto1/DiskManagement"
	"proyecto1/Structs"
	"proyecto1/Utilities"
	"fmt"
	"encoding/json"
	"net/http"
	"log"
	"os"
	"strings"
)

type CommandRequest struct {
	Command string `json:"command"`
}

type CommandResponse struct {
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

type SessionResponse struct {
	IsActive    bool   `json:"is_active"`
	Username    string `json:"username"`
	UserID      int    `json:"user_id"`
	GroupID     int    `json:"group_id"`
	PartitionID string `json:"partition_id"`
}

type LoginRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	PartitionID string `json:"partition_id"`
}

type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	User    *SessionResponse `json:"user,omitempty"`
}

type DiskInfo struct {
	Path       string          `json:"path"`
	Name       string          `json:"name"`
	Size       int32           `json:"size"`
	Fit        string          `json:"fit"`
	Partitions []PartitionInfo `json:"partitions"`
}

type PartitionInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       int32  `json:"size"`
	Fit        string `json:"fit"`
	Status     string `json:"status"`
	Start      int32  `json:"start"`
	IsMounted  bool   `json:"is_mounted"`
	MountID    string `json:"mount_id,omitempty"`
	IsLogical  bool   `json:"is_logical"`
	HasFS      bool   `json:"has_fs"` // Indica si tiene sistema de archivos (mkfs)
}

type DisksResponse struct {
	Disks []DiskInfo `json:"disks"`
}

func main() {
	fmt.Println("=== SIMULADOR DE SISTEMA DE ARCHIVOS MIA - API ===")
	fmt.Println("Servidor API iniciado en http://localhost:5000")
	fmt.Println("Endpoints disponibles:")
	fmt.Println("  POST /execute - Ejecutar comandos")
	fmt.Println("  GET  /session - Obtener estado de sesión")
	fmt.Println("  POST /login   - Iniciar sesión (solo interfaz web)")
	fmt.Println("  POST /logout  - Cerrar sesión (solo interfaz web)")
	fmt.Println("  GET  /disks   - Obtener información de discos")
	fmt.Println("================================================")

	http.HandleFunc("/execute", handleCommand)
	http.HandleFunc("/session", handleSession)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/logout", handleLogout)
	http.HandleFunc("/disks", handleDisks)
	http.HandleFunc("/filesystem/tree", handleFileSystemTree)
	http.HandleFunc("/filesystem/directory", handleDirectoryContents)
	http.HandleFunc("/filesystem/file", handleFileContent)
	http.HandleFunc("/filesystem/journaling", handleJournaling)
	http.HandleFunc("/", handleRoot)

	log.Fatal(http.ListenAndServe(":5000", nil))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{
		"message": "API del Simulador de Sistema de Archivos MIA",
		"endpoints": "POST /execute, GET /session",
		"example": `{"command": "mkdisk -size=5 -path=./test/A.mia"}`,
	}
	json.NewEncoder(w).Encode(response)
}

func handleSession(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Método no permitido. Use GET",
		})
		return
	}

	// Obtener el estado de la sesión actual
	session := FileSystem.CurrentSession
	
	var response SessionResponse
	if session != nil && session.IsActive {
		response = SessionResponse{
			IsActive:    true,
			Username:    session.Username,
			UserID:      session.UserID,
			GroupID:     session.GroupID,
			PartitionID: session.PartitionID,
		}
	} else {
		response = SessionResponse{
			IsActive: false,
		}
	}

	json.NewEncoder(w).Encode(response)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "Método no permitido. Use POST",
		})
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "Error al decodificar JSON: " + err.Error(),
		})
		return
	}

	// Validar campos requeridos
	if req.Username == "" || req.Password == "" || req.PartitionID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "Todos los campos son requeridos (username, password, partition_id)",
		})
		return
	}

	// Llamar a la función de login del FileSystem
	FileSystem.Login(req.Username, req.Password, req.PartitionID)

	// Verificar si el login fue exitoso
	session := FileSystem.CurrentSession
	if session != nil && session.IsActive {
		response := LoginResponse{
			Success: true,
			Message: "Inicio de sesión exitoso",
			User: &SessionResponse{
				IsActive:    true,
				Username:    session.Username,
				UserID:      session.UserID,
				GroupID:     session.GroupID,
				PartitionID: session.PartitionID,
			},
		}
		json.NewEncoder(w).Encode(response)
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "Credenciales incorrectas o partición no encontrada",
		})
	}
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Método no permitido. Use POST",
		})
		return
	}

	// Llamar a la función de logout del FileSystem
	FileSystem.Logout()

	// Verificar si el logout fue exitoso
	session := FileSystem.CurrentSession
	if session == nil || !session.IsActive {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Sesión cerrada exitosamente",
		})
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Error al cerrar sesión",
		})
	}
}

func handleDisks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Método no permitido. Use GET",
		})
		return
	}

	// Obtener todos los discos del mapa DrivePathMap
	var disks []DiskInfo
	diskPaths := make(map[string]bool) // Para evitar duplicados

	// Primero, obtener discos de las particiones montadas
	for _, mountedPart := range DiskManagement.MountedPartitions {
		if !diskPaths[mountedPart.Path] {
			diskPaths[mountedPart.Path] = true
		}
	}

	// También agregar discos del DrivePathMap
	for _, diskPath := range DiskManagement.DrivePathMap {
		if !diskPaths[diskPath] {
			diskPaths[diskPath] = true
		}
	}
	
	// NUEVO: También buscar discos del DiskOrderList (orden cronológico)
	for _, diskPath := range DiskManagement.DiskOrderList {
		if !diskPaths[diskPath] {
			diskPaths[diskPath] = true
		}
	}

	// Procesar cada disco único
	for diskPath := range diskPaths {
		// Verificar si el archivo existe
		if _, err := os.Stat(diskPath); os.IsNotExist(err) {
			continue
		}

		// Abrir el archivo del disco
		file, err := Utilities.OpenFile(diskPath)
		if err != nil {
			continue
		}

		// Leer el MBR
		var mbr Structs.MBR
		err = Utilities.ReadObject(file, &mbr, 0)
		if err != nil {
			file.Close()
			continue
		}

		// Extraer información del disco
		diskName := diskPath[strings.LastIndex(diskPath, "/")+1:]
		fit := strings.TrimRight(string(mbr.Fit[:]), "\x00")
		
		diskInfo := DiskInfo{
			Path:       diskPath,
			Name:       diskName,
			Size:       mbr.MbrSize,
			Fit:        fit,
			Partitions: []PartitionInfo{},
		}

		// Leer particiones primarias y extendidas
		for i := 0; i < 4; i++ {
			partition := mbr.Partitions[i]
			
			// Mostrar todas las particiones, incluso las que no tienen tamaño (para completitud)
			// Solo omitir las que realmente están vacías (size == 0)
			if partition.Size == 0 {
				continue
			}

			partName := strings.TrimRight(string(partition.Name[:]), "\x00")
			partType := string(partition.Type[0])
			partFit := strings.TrimRight(string(partition.Fit[:]), "\x00")

			// Verificar si está montada
			isMounted := false
			mountID := ""
			for id, mountedPart := range DiskManagement.MountedPartitions {
				if mountedPart.Path == diskPath && mountedPart.PartitionName == partName {
					isMounted = true
					mountID = id
					break
				}
			}

			// Determinar el estado de la partición
			status := "No montada"
			if partition.Status[0] == '1' {
				status = "Activa"
			}
			if isMounted {
				status = "Montada"
			}

			// Verificar si tiene sistema de archivos (solo para particiones primarias)
			hasFS := false
			if partType == "P" || partType == "p" {
				hasFS = hasFileSystem(file, partition.Start)
			}

			partInfo := PartitionInfo{
				Name:      partName,
				Type:      getPartitionType(partType),
				Size:      partition.Size,
				Fit:       partFit,
				Status:    status,
				Start:     partition.Start,
				IsMounted: isMounted,
				MountID:   mountID,
				IsLogical: false,
				HasFS:     hasFS,
			}

			diskInfo.Partitions = append(diskInfo.Partitions, partInfo)

			// Si es partición extendida, leer las particiones lógicas
			if partType == "E" || partType == "e" {
				logicalParts := readLogicalPartitions(file, partition.Start, diskPath, partName)
				diskInfo.Partitions = append(diskInfo.Partitions, logicalParts...)
			}
		}

		file.Close()
		disks = append(disks, diskInfo)
	}

	response := DisksResponse{
		Disks: disks,
	}

	json.NewEncoder(w).Encode(response)
}

func getPartitionType(partType string) string {
	switch partType {
	case "P", "p":
		return "Primaria"
	case "E", "e":
		return "Extendida"
	case "L", "l":
		return "Lógica"
	default:
		return "Desconocida"
	}
}

// hasFileSystem verifica si una partición tiene un sistema de archivos formateado (mkfs)
func hasFileSystem(file *os.File, partitionStart int32) bool {
	// Intentar leer el superblock de la partición
	var superblock Structs.Superblock
	err := Utilities.ReadObject(file, &superblock, int64(partitionStart))
	if err != nil {
		return false
	}

	// Verificar el número mágico del superbloque (0xEF53 para ext2/3)
	// Si el magic number es correcto, significa que tiene sistema de archivos
	if superblock.S_magic == 0xEF53 {
		return true
	}

	return false
}

func readLogicalPartitions(file *os.File, extendedStart int32, diskPath string, extendedName string) []PartitionInfo {
	var logicalParts []PartitionInfo
	ebrPosition := extendedStart

	for ebrPosition != -1 {
		var ebr Structs.EBR
		err := Utilities.ReadObject(file, &ebr, int64(ebrPosition))
		if err != nil {
			break
		}

		partName := strings.TrimRight(string(ebr.Part_name[:]), "\x00")
		// Si no hay nombre, terminar la búsqueda
		if partName == "" {
			break
		}

		// Si no hay tamaño, esta partición lógica no es válida
		if ebr.Part_size == 0 {
			if ebr.Part_next == -1 {
				break
			}
			ebrPosition = ebr.Part_next
			continue
		}

		partFit := string(ebr.Part_fit[0])

		// Verificar si está montada
		isMounted := false
		mountID := ""
		for id, mountedPart := range DiskManagement.MountedPartitions {
			if mountedPart.Path == diskPath && mountedPart.PartitionName == partName && mountedPart.IsLogical {
				isMounted = true
				mountID = id
				break
			}
		}

		// Determinar el estado de la partición
		status := "No montada"
		if ebr.Part_status[0] == '1' {
			status = "Activa"
		}
		if isMounted {
			status = "Montada"
		}

		// Verificar si tiene sistema de archivos
		hasFS := hasFileSystem(file, ebr.Part_start)

		partInfo := PartitionInfo{
			Name:      partName,
			Type:      "Lógica",
			Size:      ebr.Part_size,
			Fit:       partFit,
			Status:    status,
			Start:     ebr.Part_start,
			IsMounted: isMounted,
			MountID:   mountID,
			IsLogical: true,
			HasFS:     hasFS,
		}

		logicalParts = append(logicalParts, partInfo)

		if ebr.Part_next == -1 {
			break
		}
		ebrPosition = ebr.Part_next
	}

	return logicalParts
}

func handleCommand(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(CommandResponse{
			Error: "Método no permitido. Use POST",
		})
		return
	}

	var req CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CommandResponse{
			Error: "Error al decodificar JSON: " + err.Error(),
		})
		return
	}

	if req.Command == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(CommandResponse{
			Error: "El campo 'command' es requerido",
		})
		return
	}

	// Procesar el comando y obtener la respuesta
	output := Analyzer.ProcessCommandForAPI(req.Command)
	
	// Verificar si el cliente quiere respuesta en texto plano
	acceptHeader := r.Header.Get("Accept")
	if acceptHeader == "text/plain" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, output)
		return
	}

	// Por defecto devolver JSON pero con formato mejorado
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	response := CommandResponse{
		Output: output,
	}

	// Usar encoder con indentación para mejor legibilidad
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.Encode(response)
}

// handleFileSystemTree - Obtener el árbol completo del sistema de archivos
func handleFileSystemTree(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Método no permitido. Use GET",
		})
		return
	}

	// Obtener el ID de la partición de los parámetros de consulta
	partitionID := r.URL.Query().Get("partition_id")
	if partitionID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "El parámetro 'partition_id' es requerido",
		})
		return
	}

	// Obtener el árbol del sistema de archivos
	tree, err := FileSystem.GetFileSystemTree(partitionID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(tree)
}

// handleDirectoryContents - Obtener el contenido de un directorio
func handleDirectoryContents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Método no permitido. Use GET",
		})
		return
	}

	// Obtener parámetros
	partitionID := r.URL.Query().Get("partition_id")
	dirPath := r.URL.Query().Get("path")

	if partitionID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "El parámetro 'partition_id' es requerido",
		})
		return
	}

	if dirPath == "" {
		dirPath = "/" // Directorio raíz por defecto
	}

	// Obtener el contenido del directorio
	contents, err := FileSystem.GetDirectoryContents(partitionID, dirPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":     dirPath,
		"contents": contents,
	})
}

// handleFileContent - Obtener el contenido de un archivo
func handleFileContent(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Método no permitido. Use GET",
		})
		return
	}

	// Obtener parámetros
	partitionID := r.URL.Query().Get("partition_id")
	filePath := r.URL.Query().Get("path")

	if partitionID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "El parámetro 'partition_id' es requerido",
		})
		return
	}

	if filePath == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "El parámetro 'path' es requerido",
		})
		return
	}

	// Obtener el contenido del archivo
	content, err := FileSystem.GetFileContent(partitionID, filePath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    filePath,
		"content": content,
	})
}

// handleJournaling - Obtener las entradas del journaling
func handleJournaling(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Método no permitido",
		})
		return
	}

	// Obtener el ID de la partición de los parámetros de consulta
	partitionID := r.URL.Query().Get("id")
	if partitionID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "El parámetro 'id' es requerido",
		})
		return
	}

	// Obtener las entradas del journaling
	entries, err := FileSystem.GetJournalingData(partitionID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"partition_id": partitionID,
		"entries":      entries,
		"total":        len(entries),
	})
}
