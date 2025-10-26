package Reportes

import (
	"proyecto1/DiskManagement"
	"proyecto1/Structs"
	"proyecto1/Utilities"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GenerateMBRReport genera el reporte MBR en formato Graphviz DOT e imagen
func GenerateMBRReport(userOutputPath string, partitionID string) error {
	fmt.Println("=== GENERANDO REPORTE MBR ===")
	fmt.Printf("Ruta de salida especificada: %s\n", userOutputPath)
	fmt.Printf("ID de partición: %s\n", partitionID)
	
	// Buscar la partición montada para obtener la ruta del disco
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return fmt.Errorf("la partición con ID '%s' no está montada", partitionID)
	}

	diskPath := mountedPartition.Path
	fmt.Printf("Ruta del disco: %s\n", diskPath)

	// Abrir archivo del disco
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("error abriendo archivo del disco: %v", err)
	}
	defer file.Close()

	// Leer MBR
	var mbr Structs.MBR
	if err := Utilities.ReadObject(file, &mbr, 0); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Usar la ruta exacta especificada por el usuario
	finalDotPath, finalImagePath := processUserPath(userOutputPath)
	fmt.Printf("Archivo DOT: %s\n", finalDotPath)
	fmt.Printf("Archivo imagen: %s\n", finalImagePath)

	// Crear directorio de salida si no existe
	if err := createOutputDirectory(finalDotPath); err != nil {
		return fmt.Errorf("error creando directorio de salida: %v", err)
	}

	// Generar contenido del reporte en formato DOT
	dotContent := generateMBRDotContent(file, &mbr)

	// Escribir archivo DOT
	if err := writeReportFile(finalDotPath, dotContent); err != nil {
		return fmt.Errorf("error escribiendo archivo DOT: %v", err)
	}

	// Generar imagen usando Graphviz
	if err := generateGraphvizImage(finalDotPath, finalImagePath); err != nil {
		fmt.Printf("Advertencia: No se pudo generar la imagen: %v\n", err)
		fmt.Println("Asegúrate de tener Graphviz instalado (sudo apt install graphviz)")
	} else {
		fmt.Printf("✓ Imagen generada: %s\n", finalImagePath)
	}

	fmt.Printf("✓ Reporte MBR generado exitosamente\n")
	fmt.Printf("  - Archivo DOT: %s\n", finalDotPath)
	fmt.Printf("  - Archivo imagen: %s\n", finalImagePath)
	return nil
}

// generateMBRDotContent genera el contenido del reporte MBR en formato DOT
func generateMBRDotContent(file *os.File, mbr *Structs.MBR) string {
	var content strings.Builder

	// Encabezado del archivo DOT
	content.WriteString("digraph MBR_Report {\n")
	content.WriteString("    rankdir=TB;\n")
	content.WriteString("    node [shape=plaintext];\n")
	content.WriteString("    \n")

	// Tabla principal del MBR
	content.WriteString("    mbr_table [label=<\n")
	content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
	content.WriteString("            <TR><TD COLSPAN=\"2\" BGCOLOR=\"#4CAF50\"><B>REPORTE MBR</B></TD></TR>\n")
	
	// Información general del MBR
	content.WriteString(fmt.Sprintf("            <TR><TD><B>Tamaño MBR</B></TD><TD>%d bytes</TD></TR>\n", mbr.MbrSize))
	content.WriteString(fmt.Sprintf("            <TR><TD><B>Fecha de Creación</B></TD><TD>%s</TD></TR>\n", cleanString(mbr.CreationDate[:])))
	content.WriteString(fmt.Sprintf("            <TR><TD><B>Signature</B></TD><TD>%d</TD></TR>\n", mbr.Signature))
	content.WriteString(fmt.Sprintf("            <TR><TD><B>Fit</B></TD><TD>%s</TD></TR>\n", cleanString(mbr.Fit[:])))
	
	// Espacio usado por MBR
	mbrSize := int32(binary.Size(*mbr))
	content.WriteString(fmt.Sprintf("            <TR><TD><B>Tamaño Estructura MBR</B></TD><TD>%d bytes</TD></TR>\n", mbrSize))
	
	content.WriteString("        </TABLE>\n")
	content.WriteString("    >];\n\n")

	// Tabla de particiones
	content.WriteString("    partitions_table [label=<\n")
	content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
	content.WriteString("            <TR><TD COLSPAN=\"7\" BGCOLOR=\"#2196F3\"><B>PARTICIONES</B></TD></TR>\n")
	content.WriteString("            <TR>\n")
	content.WriteString("                <TD><B>Nombre</B></TD>\n")
	content.WriteString("                <TD><B>Tipo</B></TD>\n") 
	content.WriteString("                <TD><B>Estado</B></TD>\n")
	content.WriteString("                <TD><B>Inicio</B></TD>\n")
	content.WriteString("                <TD><B>Tamaño</B></TD>\n")
	content.WriteString("                <TD><B>Fit</B></TD>\n")
	content.WriteString("                <TD><B>ID</B></TD>\n")
	content.WriteString("            </TR>\n")

	// Iterar por las 4 particiones del MBR
	extendedPartitionIndex := -1
	for i := 0; i < 4; i++ {
		partition := &mbr.Partitions[i]
		
		if partition.Size > 0 {
			// Determinar color de fondo según el tipo
			bgColor := "#FFFFFF"
			partitionType := cleanString(partition.Type[:])
			switch partitionType {
			case "p":
				bgColor = "#E8F5E8"
			case "e":
				bgColor = "#FFF3E0"
				extendedPartitionIndex = i
			}

			content.WriteString(fmt.Sprintf("            <TR BGCOLOR=\"%s\">\n", bgColor))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", cleanString(partition.Name[:])))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", getPartitionTypeText(partitionType)))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", getStatusText(cleanString(partition.Status[:]))))
			content.WriteString(fmt.Sprintf("                <TD>%d</TD>\n", partition.Start))
			content.WriteString(fmt.Sprintf("                <TD>%d bytes</TD>\n", partition.Size))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", cleanString(partition.Fit[:])))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", cleanString(partition.Id[:])))
			content.WriteString("            </TR>\n")
		} else {
			// Partición vacía
			content.WriteString("            <TR>\n")
			content.WriteString("                <TD>-</TD>\n")
			content.WriteString("                <TD>Vacía</TD>\n")
			content.WriteString("                <TD>-</TD>\n")
			content.WriteString("                <TD>-</TD>\n")
			content.WriteString("                <TD>-</TD>\n")
			content.WriteString("                <TD>-</TD>\n")
			content.WriteString("                <TD>-</TD>\n")
			content.WriteString("            </TR>\n")
		}
	}

	content.WriteString("        </TABLE>\n")
	content.WriteString("    >];\n\n")

	// Si hay partición extendida, mostrar las particiones lógicas (EBRs)
	if extendedPartitionIndex != -1 {
		ebrContent := generateEBRContent(file, &mbr.Partitions[extendedPartitionIndex])
		content.WriteString(ebrContent)
		
		// Conexión visual entre particiones y EBRs
		content.WriteString("    partitions_table -> ebr_table [style=dashed, color=blue, label=\"Particiones Lógicas\"];\n")
	}

	// Conexión visual entre MBR y particiones
	content.WriteString("    mbr_table -> partitions_table [color=green, label=\"Contiene\"];\n")

	// Cierre del archivo DOT
	content.WriteString("}\n")

	return content.String()
}

// generateEBRContent genera el contenido de los EBRs (particiones lógicas)
func generateEBRContent(file *os.File, extendedPartition *Structs.Partition) string {
	var content strings.Builder

	// Tabla de EBRs
	content.WriteString("    ebr_table [label=<\n")
	content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
	content.WriteString("            <TR><TD COLSPAN=\"6\" BGCOLOR=\"#FF9800\"><B>PARTICIONES LÓGICAS (EBRs)</B></TD></TR>\n")
	content.WriteString("            <TR>\n")
	content.WriteString("                <TD><B>Nombre</B></TD>\n")
	content.WriteString("                <TD><B>Estado</B></TD>\n")
	content.WriteString("                <TD><B>Inicio</B></TD>\n")
	content.WriteString("                <TD><B>Tamaño</B></TD>\n")
	content.WriteString("                <TD><B>Siguiente</B></TD>\n")
	content.WriteString("                <TD><B>Fit</B></TD>\n")
	content.WriteString("            </TR>\n")

	// Navegar por la lista de EBRs
	currentEBRPos := extendedPartition.Start
	ebrCount := 0

	for {
		var currentEBR Structs.EBR
		if err := Utilities.ReadObject(file, &currentEBR, int64(currentEBRPos)); err != nil {
			fmt.Printf("Error leyendo EBR en posición %d: %v\n", currentEBRPos, err)
			break
		}

		// Si el EBR tiene tamaño > 0, es una partición lógica válida
		if currentEBR.Part_size > 0 {
			ebrCount++
			content.WriteString("            <TR BGCOLOR=\"#FFF8E1\">\n")
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", cleanString(currentEBR.Part_name[:])))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", getStatusText(cleanString(currentEBR.Part_status[:]))))
			content.WriteString(fmt.Sprintf("                <TD>%d</TD>\n", currentEBR.Part_start))
			content.WriteString(fmt.Sprintf("                <TD>%d bytes</TD>\n", currentEBR.Part_size))
			if currentEBR.Part_next == -1 {
				content.WriteString("                <TD>Final</TD>\n")
			} else {
				content.WriteString(fmt.Sprintf("                <TD>%d</TD>\n", currentEBR.Part_next))
			}
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", cleanString(currentEBR.Part_fit[:])))
			content.WriteString("            </TR>\n")
		}

		// Si no hay siguiente EBR, terminar
		if currentEBR.Part_next == -1 {
			break
		}

		currentEBRPos = currentEBR.Part_next
	}

	// Si no hay particiones lógicas
	if ebrCount == 0 {
		content.WriteString("            <TR>\n")
		content.WriteString("                <TD COLSPAN=\"6\">No hay particiones lógicas</TD>\n")
		content.WriteString("            </TR>\n")
	}

	content.WriteString("        </TABLE>\n")
	content.WriteString("    >];\n\n")

	return content.String()
}

// Funciones auxiliares

func cleanString(data []byte) string {
	// Convertir bytes a string y eliminar caracteres nulos
	str := string(data)
	// Encontrar el primer byte nulo
	if idx := strings.IndexByte(str, 0); idx != -1 {
		str = str[:idx]
	}
	return strings.TrimSpace(str)
}

func getPartitionTypeText(typeCode string) string {
	switch strings.ToLower(typeCode) {
	case "p":
		return "Primaria"
	case "e":
		return "Extendida"
	case "l":
		return "Lógica"
	default:
		return typeCode
	}
}

func getStatusText(statusCode string) string {
	switch statusCode {
	case "0":
		return "Inactiva"
	case "1":
		return "Activa"
	default:
		return statusCode
	}
}

func createOutputDirectory(outputPath string) error {
	dir := filepath.Dir(outputPath)
	if dir != "." && dir != "" {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				return fmt.Errorf("error creando directorio %s: %v", dir, err)
			}
			fmt.Printf("Directorio creado: %s\n", dir)
		}
	}
	return nil
}

func writeReportFile(outputPath string, content string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}

// processUserPath procesa la ruta del usuario y genera siempre rutas .dot y .jpg
func processUserPath(userOutputPath string) (string, string) {
	// Limpiar la ruta del usuario (remover comillas si las tiene)
	cleanPath := strings.Trim(userOutputPath, "\"")
	
	// Eliminar extensión si la tuviera
	basePath := strings.TrimSuffix(cleanPath, filepath.Ext(cleanPath))
	
	// Siempre generar .dot y .jpg
	dotPath := basePath + ".dot"
	imagePath := basePath + ".jpg"
	
	return dotPath, imagePath
}

// generateGraphvizImage genera siempre una imagen JPG desde un archivo DOT
func generateGraphvizImage(dotPath string, imagePath string) error {
	// Verificar que el archivo DOT existe
	if _, err := os.Stat(dotPath); os.IsNotExist(err) {
		return fmt.Errorf("el archivo DOT no existe: %s", dotPath)
	}
	
	// Siempre JPG
	cmd := exec.Command("dot", "-Tjpg", dotPath, "-o", imagePath)
	
	// Ejecutar el comando
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error ejecutando Graphviz: %v\nOutput: %s", err, string(output))
	}
	
	// Verificar que se creó la imagen
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return fmt.Errorf("la imagen no se generó correctamente: %s", imagePath)
	}
	
	return nil
}

// GenerateDiskReport genera el reporte DISK en formato Graphviz DOT e imagen
func GenerateDiskReport(userOutputPath string, partitionID string) error {
	fmt.Println("=== GENERANDO REPORTE DISK ===")
	fmt.Printf("Ruta de salida especificada: %s\n", userOutputPath)
	fmt.Printf("ID de partición: %s\n", partitionID)
	
	// Buscar la partición montada para obtener la ruta del disco
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return fmt.Errorf("la partición con ID '%s' no está montada", partitionID)
	}

	diskPath := mountedPartition.Path
	fmt.Printf("Ruta del disco: %s\n", diskPath)

	// Abrir archivo del disco
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("error abriendo archivo del disco: %v", err)
	}
	defer file.Close()

	// Leer MBR
	var mbr Structs.MBR
	if err := Utilities.ReadObject(file, &mbr, 0); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// El tamaño total del disco está en el MBR
	diskSize := int64(mbr.MbrSize)

	// Usar la ruta exacta especificada por el usuario
	finalDotPath, finalImagePath := processUserPath(userOutputPath)
	fmt.Printf("Archivo DOT: %s\n", finalDotPath)
	fmt.Printf("Archivo imagen: %s\n", finalImagePath)

	// Crear directorio de salida si no existe
	if err := createOutputDirectory(finalDotPath); err != nil {
		return fmt.Errorf("error creando directorio de salida: %v", err)
	}

	// Generar contenido del reporte en formato DOT
	dotContent := generateDiskDotContent(file, &mbr, diskSize)

	// Escribir archivo DOT
	if err := writeReportFile(finalDotPath, dotContent); err != nil {
		return fmt.Errorf("error escribiendo archivo DOT: %v", err)
	}

	// Generar imagen usando Graphviz
	if err := generateGraphvizImage(finalDotPath, finalImagePath); err != nil {
		fmt.Printf("Advertencia: No se pudo generar la imagen: %v\n", err)
		fmt.Println("Asegúrate de tener Graphviz instalado (sudo apt install graphviz)")
	} else {
		fmt.Printf("✓ Imagen generada: %s\n", finalImagePath)
	}

	fmt.Printf("✓ Reporte DISK generado exitosamente\n")
	fmt.Printf("  - Archivo DOT: %s\n", finalDotPath)
	fmt.Printf("  - Archivo imagen: %s\n", finalImagePath)
	return nil
}

// generateDiskDotContent genera el contenido del reporte DISK en formato DOT
func generateDiskDotContent(file *os.File, mbr *Structs.MBR, diskSize int64) string {
	var content strings.Builder

	// Encabezado del archivo DOT
	content.WriteString("digraph disk_partitions {\n")
	content.WriteString("    // Configuración general\n")
	content.WriteString("    node [shape=record, style=filled, fontname=\"Arial\", fontsize=10];\n")
	content.WriteString("    \n")

	// Calcular estructura del disco y porcentajes
	diskStructure := calculateDiskStructure(file, mbr, diskSize)
	
	// Generar el label del nodo principal
	diskLabel := buildDiskLabel(diskStructure)
	
	// Nodo principal con la estructura del disco
	content.WriteString("    disk [label=\"")
	content.WriteString(diskLabel)
	content.WriteString("\",\n")
	content.WriteString("          fillcolor=lightblue,\n")
	content.WriteString("          style=\"filled,bold\"];\n")
	content.WriteString("    \n")

	// Título
	content.WriteString("    // Título\n")
	content.WriteString("    label=\"Reporte de Estructura de Particiones del Disco\";\n")
	content.WriteString("    labelloc=top;\n")
	content.WriteString("    fontsize=14;\n")
	content.WriteString("    fontname=\"Arial Bold\";\n")

	// Cierre del archivo DOT
	content.WriteString("}\n")

	return content.String()
}

// DiskSegment representa un segmento del disco (partición o espacio libre)
type DiskSegment struct {
	Type        string  // "MBR", "Primary", "Extended", "Logical", "Free"
	Name        string  // Nombre de la partición o descripción
	Start       int32   // Inicio del segmento
	Size        int32   // Tamaño del segmento
	Percentage  float64 // Porcentaje del disco total
	IsContainer bool    // true si es un contenedor (partición extendida)
	Children    []DiskSegment // Segmentos hijos (para particiones extendidas)
}

// calculateDiskStructure calcula la estructura del disco y los porcentajes
func calculateDiskStructure(file *os.File, mbr *Structs.MBR, diskSize int64) []DiskSegment {
	var segments []DiskSegment
	
	// Agregar el MBR
	mbrSize := int32(binary.Size(*mbr))
	segments = append(segments, DiskSegment{
		Type:       "MBR",
		Name:       "MBR",
		Start:      0,
		Size:       mbrSize,
		Percentage: float64(mbrSize) / float64(diskSize) * 100,
	})

	// Recopilar todas las particiones activas y ordenarlas por posición
	var activePartitions []struct {
		Partition *Structs.Partition
		Index     int
	}

	for i := 0; i < 4; i++ {
		if mbr.Partitions[i].Size > 0 {
			activePartitions = append(activePartitions, struct {
				Partition *Structs.Partition
				Index     int
			}{&mbr.Partitions[i], i})
		}
	}

	// Ordenar por posición de inicio
	for i := 0; i < len(activePartitions); i++ {
		for j := i + 1; j < len(activePartitions); j++ {
			if activePartitions[i].Partition.Start > activePartitions[j].Partition.Start {
				activePartitions[i], activePartitions[j] = activePartitions[j], activePartitions[i]
			}
		}
	}

	// Procesar particiones y espacios libres
	currentPos := mbrSize
	
	for _, ap := range activePartitions {
		partition := ap.Partition
		
		// Agregar espacio libre antes de la partición si existe
		if partition.Start > currentPos {
			freeSize := partition.Start - currentPos
			segments = append(segments, DiskSegment{
				Type:       "Free",
				Name:       "Libre",
				Start:      currentPos,
				Size:       freeSize,
				Percentage: float64(freeSize) / float64(diskSize) * 100,
			})
		}

		partitionName := cleanString(partition.Name[:])
		partitionType := cleanString(partition.Type[:])
		
		if partitionType == "p" {
			// Partición primaria
			segments = append(segments, DiskSegment{
				Type:       "Primary",
				Name:       partitionName,
				Start:      partition.Start,
				Size:       partition.Size,
				Percentage: float64(partition.Size) / float64(diskSize) * 100,
			})
		} else if partitionType == "e" {
			// Partición extendida - procesar las particiones lógicas
			logicalSegments := processExtendedPartition(file, partition, diskSize)
			segments = append(segments, DiskSegment{
				Type:        "Extended",
				Name:        partitionName,
				Start:       partition.Start,
				Size:        partition.Size,
				Percentage:  float64(partition.Size) / float64(diskSize) * 100,
				IsContainer: true,
				Children:    logicalSegments,
			})
		}
		
		currentPos = partition.Start + partition.Size
	}

	// Agregar espacio libre al final si existe
	if currentPos < int32(diskSize) {
		freeSize := int32(diskSize) - currentPos
		segments = append(segments, DiskSegment{
			Type:       "Free",
			Name:       "Libre",
			Start:      currentPos,
			Size:       freeSize,
			Percentage: float64(freeSize) / float64(diskSize) * 100,
		})
	}

	return segments
}

// processExtendedPartition procesa una partición extendida y retorna sus particiones lógicas
func processExtendedPartition(file *os.File, extPartition *Structs.Partition, diskSize int64) []DiskSegment {
	var logicalSegments []DiskSegment
	
	// Navegar por la lista de EBRs
	currentEBRPos := extPartition.Start
	extendedStart := extPartition.Start
	
	for {
		var currentEBR Structs.EBR
		if err := Utilities.ReadObject(file, &currentEBR, int64(currentEBRPos)); err != nil {
			break
		}

		// Si hay espacio libre antes de esta partición lógica
		if len(logicalSegments) == 0 && currentEBR.Part_start > currentEBRPos {
			// Espacio libre inicial
			freeSize := currentEBR.Part_start - currentEBRPos
			logicalSegments = append(logicalSegments, DiskSegment{
				Type:       "Free",
				Name:       "Libre",
				Start:      currentEBRPos,
				Size:       freeSize,
				Percentage: float64(freeSize) / float64(diskSize) * 100,
			})
		}

		// Agregar el EBR
		ebrSize := int32(binary.Size(currentEBR))
		logicalSegments = append(logicalSegments, DiskSegment{
			Type:       "EBR",
			Name:       "EBR",
			Start:      currentEBRPos,
			Size:       ebrSize,
			Percentage: float64(ebrSize) / float64(diskSize) * 100,
		})

		// Si el EBR tiene una partición lógica válida
		if currentEBR.Part_size > 0 {
			logicalName := cleanString(currentEBR.Part_name[:])
			logicalSegments = append(logicalSegments, DiskSegment{
				Type:       "Logical",
				Name:       logicalName,
				Start:      currentEBR.Part_start,
				Size:       currentEBR.Part_size,
				Percentage: float64(currentEBR.Part_size) / float64(diskSize) * 100,
			})
		}

		// Si no hay siguiente EBR, agregar espacio libre restante
		if currentEBR.Part_next == -1 {
			lastPos := currentEBR.Part_start + currentEBR.Part_size
			extendedEnd := extendedStart + extPartition.Size
			if lastPos < extendedEnd {
				freeSize := extendedEnd - lastPos
				logicalSegments = append(logicalSegments, DiskSegment{
					Type:       "Free",
					Name:       "Libre",
					Start:      lastPos,
					Size:       freeSize,
					Percentage: float64(freeSize) / float64(diskSize) * 100,
				})
			}
			break
		}

		currentEBRPos = currentEBR.Part_next
	}

	return logicalSegments
}

// buildDiskLabel construye el label del nodo principal para Graphviz
func buildDiskLabel(segments []DiskSegment) string {
	var parts []string
	
	for _, segment := range segments {
		if segment.IsContainer {
			// Partición extendida con sus hijos
			var childParts []string
			for _, child := range segment.Children {
				childLabel := fmt.Sprintf("%s\\n%.1f%% del disco", child.Name, child.Percentage)
				childParts = append(childParts, childLabel)
			}
			
			extendedLabel := fmt.Sprintf("{%s|{%s}}", segment.Name, strings.Join(childParts, "|"))
			parts = append(parts, extendedLabel)
		} else {
			// Partición simple o espacio libre
			segmentLabel := fmt.Sprintf("%s\\n%.1f%% del disco", segment.Name, segment.Percentage)
			parts = append(parts, segmentLabel)
		}
	}
	
	return "{" + strings.Join(parts, "|") + "}"
}

// GenerateInodeReport genera el reporte de inodos en formato Graphviz DOT e imagen
func GenerateInodeReport(userOutputPath string, partitionID string) error {
	fmt.Println("=== GENERANDO REPORTE INODE ===")
	fmt.Printf("Ruta de salida especificada: %s\n", userOutputPath)
	fmt.Printf("ID de partición: %s\n", partitionID)
	
	// Buscar la partición montada para obtener la ruta del disco
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return fmt.Errorf("la partición con ID '%s' no está montada", partitionID)
	}

	diskPath := mountedPartition.Path
	fmt.Printf("Ruta del disco: %s\n", diskPath)

	// Abrir archivo del disco
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("error abriendo archivo del disco: %v", err)
	}
	defer file.Close()

	// Leer el superblock para obtener la estructura del sistema de archivos
	var tempMBR Structs.MBR
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Obtener la partición correcta
	var partition *Structs.Partition = nil
	if !mountedPartition.IsLogical {
		partition = &tempMBR.Partitions[mountedPartition.PartitionIndex]
	} else {
		// Para partición lógica, crear una partición temporal
		var tempEBR Structs.EBR
		if err := Utilities.ReadObject(file, &tempEBR, int64(mountedPartition.EBRPosition)); err != nil {
			return fmt.Errorf("error leyendo EBR: %v", err)
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
		return fmt.Errorf("error leyendo superblock: %v", err)
	}

	// Usar la ruta exacta especificada por el usuario
	finalDotPath, finalImagePath := processUserPath(userOutputPath)
	fmt.Printf("Archivo DOT: %s\n", finalDotPath)
	fmt.Printf("Archivo imagen: %s\n", finalImagePath)

	// Crear directorio de salida si no existe
	if err := createOutputDirectory(finalDotPath); err != nil {
		return fmt.Errorf("error creando directorio de salida: %v", err)
	}

	// Generar contenido del reporte en formato DOT
	dotContent := generateInodeDotContent(file, &superblock)

	// Escribir archivo DOT
	if err := writeReportFile(finalDotPath, dotContent); err != nil {
		return fmt.Errorf("error escribiendo archivo DOT: %v", err)
	}

	// Generar imagen usando Graphviz
	if err := generateGraphvizImage(finalDotPath, finalImagePath); err != nil {
		fmt.Printf("Advertencia: No se pudo generar la imagen: %v\n", err)
		fmt.Println("Asegúrate de tener Graphviz instalado (sudo apt install graphviz)")
	} else {
		fmt.Printf("✓ Imagen generada: %s\n", finalImagePath)
	}

	fmt.Printf("✓ Reporte INODE generado exitosamente\n")
	fmt.Printf("  - Archivo DOT: %s\n", finalDotPath)
	fmt.Printf("  - Archivo imagen: %s\n", finalImagePath)
	return nil
}

// generateInodeDotContent genera el contenido del reporte INODE en formato DOT
func generateInodeDotContent(file *os.File, superblock *Structs.Superblock) string {
	var content strings.Builder

	// Encabezado del archivo DOT
	content.WriteString("digraph inode_report {\n")
	content.WriteString("    // Configuración general\n")
	content.WriteString("    node [shape=plaintext, fontname=\"Arial\", fontsize=10];\n")
	content.WriteString("    rankdir=LR;\n")
	content.WriteString("    \n")

	// Título del reporte
	content.WriteString("    // Título\n")
	content.WriteString("    title [label=\"REPORTE DE INODOS\", fontsize=16, fontname=\"Arial Bold\", color=blue];\n")
	content.WriteString("    \n")

	usedInodes := 0
	
	// Iterar por todos los inodos para encontrar los utilizados
	for i := int32(0); i < superblock.S_inodes_count; i++ {
		// Verificar si el inodo está en uso (bitmap)
		var bitmapByte byte
		if err := Utilities.ReadObject(file, &bitmapByte, int64(superblock.S_bm_inode_start+i)); err != nil {
			continue // Error leyendo bitmap, continuar
		}
		
		// Si el inodo no está en uso, continuar
		if bitmapByte == 0 {
			continue
		}

		// Leer el inodo
		var inode Structs.Inode
		inodePos := int64(superblock.S_inode_start + i*superblock.S_inode_size)
		if err := Utilities.ReadObject(file, &inode, inodePos); err != nil {
			continue // Error leyendo inodo, continuar
		}

		usedInodes++

		// Generar tabla para este inodo
		content.WriteString(fmt.Sprintf("    inode_%d [label=<\n", i))
		content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
		
		// Encabezado del inodo con color según el tipo
		inodeType := cleanString(inode.I_type[:])
		bgColor := "#E3F2FD"
		typeText := "Archivo"
		if inodeType == "0" {
			bgColor = "#FFF3E0"
			typeText = "Directorio"
		}
		
		content.WriteString(fmt.Sprintf("            <TR><TD COLSPAN=\"2\" BGCOLOR=\"%s\"><B>INODO %d (%s)</B></TD></TR>\n", bgColor, i, typeText))
		
		// Información básica del inodo
		content.WriteString(fmt.Sprintf("            <TR><TD><B>UID</B></TD><TD>%d</TD></TR>\n", inode.I_uid))
		content.WriteString(fmt.Sprintf("            <TR><TD><B>GID</B></TD><TD>%d</TD></TR>\n", inode.I_gid))
		content.WriteString(fmt.Sprintf("            <TR><TD><B>Tamaño</B></TD><TD>%d bytes</TD></TR>\n", inode.I_size))
		content.WriteString(fmt.Sprintf("            <TR><TD><B>Tiempo Acceso</B></TD><TD>%s</TD></TR>\n", cleanString(inode.I_atime[:])))
		content.WriteString(fmt.Sprintf("            <TR><TD><B>Tiempo Creación</B></TD><TD>%s</TD></TR>\n", cleanString(inode.I_ctime[:])))
		content.WriteString(fmt.Sprintf("            <TR><TD><B>Tiempo Modif.</B></TD><TD>%s</TD></TR>\n", cleanString(inode.I_mtime[:])))
		content.WriteString(fmt.Sprintf("            <TR><TD><B>Tipo</B></TD><TD>%s</TD></TR>\n", inodeType))
		content.WriteString(fmt.Sprintf("            <TR><TD><B>Permisos</B></TD><TD>%s</TD></TR>\n", cleanString(inode.I_perm[:])))
		
		// Mostrar punteros a bloques (solo los que están en uso)
		content.WriteString("            <TR><TD COLSPAN=\"2\" BGCOLOR=\"#F5F5F5\"><B>PUNTEROS A BLOQUES</B></TD></TR>\n")
		
		// Punteros directos (0-11)
		directBlocks := 0
		for j := 0; j < 12; j++ {
			if inode.I_block[j] != -1 {
				content.WriteString(fmt.Sprintf("            <TR><TD>Directo %d</TD><TD>%d</TD></TR>\n", j, inode.I_block[j]))
				directBlocks++
			}
		}
		
		// Puntero indirecto simple (12)
		if len(inode.I_block) > 12 && inode.I_block[12] != -1 {
			content.WriteString(fmt.Sprintf("            <TR><TD>Indirecto Simple</TD><TD>%d</TD></TR>\n", inode.I_block[12]))
			
			// Leer el bloque de punteros indirectos y mostrar algunos punteros
			var indirectBlock Structs.Fileblock
			indirectBlockPos := int64(superblock.S_block_start + inode.I_block[12]*superblock.S_block_size)
			if err := Utilities.ReadObject(file, &indirectBlock, indirectBlockPos); err == nil {
				indirectCount := 0
				for k := 0; k < 16 && indirectCount < 5; k++ { // Mostrar solo los primeros 5 para no sobrecargar
					var blockNumber int32
					binary.Read(strings.NewReader(string(indirectBlock.B_content[k*4:(k+1)*4])), binary.LittleEndian, &blockNumber)
					if blockNumber != -1 {
						content.WriteString(fmt.Sprintf("            <TR><TD>  → Indirecto[%d]</TD><TD>%d</TD></TR>\n", k, blockNumber))
						indirectCount++
					}
				}
				if indirectCount == 5 {
					content.WriteString("            <TR><TD COLSPAN=\"2\">  → ...</TD></TR>\n")
				}
			}
		}
		
		// Puntero indirecto doble (13) - si existe
		if len(inode.I_block) > 13 && inode.I_block[13] != -1 {
			content.WriteString(fmt.Sprintf("            <TR><TD>Indirecto Doble</TD><TD>%d</TD></TR>\n", inode.I_block[13]))
		}
		
		// Puntero indirecto triple (14) - si existe
		if len(inode.I_block) > 14 && inode.I_block[14] != -1 {
			content.WriteString(fmt.Sprintf("            <TR><TD>Indirecto Triple</TD><TD>%d</TD></TR>\n", inode.I_block[14]))
		}
		
		// Si no hay bloques asignados
		if directBlocks == 0 && (len(inode.I_block) <= 12 || inode.I_block[12] == -1) {
			content.WriteString("            <TR><TD COLSPAN=\"2\">Sin bloques asignados</TD></TR>\n")
		}
		
		content.WriteString("        </TABLE>\n")
		content.WriteString("    >];\n\n")
	}

	// Si no hay inodos utilizados
	if usedInodes == 0 {
		content.WriteString("    no_inodes [label=<\n")
		content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
		content.WriteString("            <TR><TD BGCOLOR=\"#FFCDD2\"><B>NO HAY INODOS UTILIZADOS</B></TD></TR>\n")
		content.WriteString("            <TR><TD>El sistema de archivos no tiene inodos en uso</TD></TR>\n")
		content.WriteString("        </TABLE>\n")
		content.WriteString("    >];\n")
		content.WriteString("    title -> no_inodes;\n")
	} else {
		// Conectar título con el primer inodo para organización visual
		content.WriteString("    title -> inode_0 [style=invis];\n")
		
		// Organizar inodos en línea (algunos de ellos)
		if usedInodes > 1 {
			content.WriteString("    // Organización visual\n")
			content.WriteString("    {rank=same; ")
			nodeCount := 0
			for i := int32(0); i < superblock.S_inodes_count && nodeCount < 4; i++ {
				var bitmapByte byte
				if err := Utilities.ReadObject(file, &bitmapByte, int64(superblock.S_bm_inode_start+i)); err != nil {
					continue
				}
				if bitmapByte != 0 {
					content.WriteString(fmt.Sprintf("inode_%d; ", i))
					nodeCount++
				}
			}
			content.WriteString("}\n")
		}
	}

	// Información del reporte
	content.WriteString("    \n")
	content.WriteString("    // Información del reporte\n")
	content.WriteString("    info [label=<\n")
	content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
	content.WriteString("            <TR><TD COLSPAN=\"2\" BGCOLOR=\"#E8F5E8\"><B>INFORMACIÓN DEL REPORTE</B></TD></TR>\n")
	content.WriteString(fmt.Sprintf("            <TR><TD><B>Total Inodos</B></TD><TD>%d</TD></TR>\n", superblock.S_inodes_count))
	content.WriteString(fmt.Sprintf("            <TR><TD><B>Inodos Utilizados</B></TD><TD>%d</TD></TR>\n", usedInodes))
	content.WriteString(fmt.Sprintf("            <TR><TD><B>Inodos Libres</B></TD><TD>%d</TD></TR>\n", superblock.S_free_inodes_count))
	content.WriteString(fmt.Sprintf("            <TR><TD><B>Primer Inodo Libre</B></TD><TD>%d</TD></TR>\n", superblock.S_fist_ino))
	content.WriteString("        </TABLE>\n")
	content.WriteString("    >];\n")

	// Cierre del archivo DOT
	content.WriteString("}\n")

	fmt.Printf("Inodos procesados: %d utilizados de %d totales\n", usedInodes, superblock.S_inodes_count)
	return content.String()
}

// GenerateBlockReport genera el reporte de bloques en formato Graphviz DOT e imagen
func GenerateBlockReport(userOutputPath string, partitionID string) error {
	fmt.Println("=== GENERANDO REPORTE BLOCK ===")
	fmt.Printf("Ruta de salida especificada: %s\n", userOutputPath)
	fmt.Printf("ID de partición: %s\n", partitionID)
	
	// Buscar la partición montada para obtener la ruta del disco
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return fmt.Errorf("la partición con ID '%s' no está montada", partitionID)
	}

	diskPath := mountedPartition.Path
	fmt.Printf("Ruta del disco: %s\n", diskPath)

	// Abrir archivo del disco
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("error abriendo archivo del disco: %v", err)
	}
	defer file.Close()

	// Leer el superblock para obtener la estructura del sistema de archivos
	var tempMBR Structs.MBR
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Obtener la partición correcta
	var partition *Structs.Partition = nil
	if !mountedPartition.IsLogical {
		partition = &tempMBR.Partitions[mountedPartition.PartitionIndex]
	} else {
		// Para partición lógica, crear una partición temporal
		var tempEBR Structs.EBR
		if err := Utilities.ReadObject(file, &tempEBR, int64(mountedPartition.EBRPosition)); err != nil {
			return fmt.Errorf("error leyendo EBR: %v", err)
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
		return fmt.Errorf("error leyendo superblock: %v", err)
	}

	// Usar la ruta exacta especificada por el usuario
	finalDotPath, finalImagePath := processUserPath(userOutputPath)
	fmt.Printf("Archivo DOT: %s\n", finalDotPath)
	fmt.Printf("Archivo imagen: %s\n", finalImagePath)

	// Crear directorio de salida si no existe
	if err := createOutputDirectory(finalDotPath); err != nil {
		return fmt.Errorf("error creando directorio de salida: %v", err)
	}

	// Generar contenido del reporte en formato DOT
	dotContent := generateBlockDotContent(file, &superblock)

	// Escribir archivo DOT
	if err := writeReportFile(finalDotPath, dotContent); err != nil {
		return fmt.Errorf("error escribiendo archivo DOT: %v", err)
	}

	// Generar imagen usando Graphviz
	if err := generateGraphvizImage(finalDotPath, finalImagePath); err != nil {
		fmt.Printf("Advertencia: No se pudo generar la imagen: %v\n", err)
		fmt.Println("Asegúrate de tener Graphviz instalado (sudo apt install graphviz)")
	} else {
		fmt.Printf("✓ Imagen generada: %s\n", finalImagePath)
	}

	fmt.Printf("✓ Reporte BLOCK generado exitosamente\n")
	fmt.Printf("  - Archivo DOT: %s\n", finalDotPath)
	fmt.Printf("  - Archivo imagen: %s\n", finalImagePath)
	return nil
}

// generateBlockDotContent genera el contenido del reporte BLOCK en formato DOT
func generateBlockDotContent(file *os.File, superblock *Structs.Superblock) string {
	var content strings.Builder

	// Encabezado del archivo DOT
	content.WriteString("digraph block_report {\n")
	content.WriteString("    // Configuración general\n")
	content.WriteString("    node [shape=plaintext, fontname=\"Arial\", fontsize=9];\n")
	content.WriteString("    rankdir=TB;\n")
	content.WriteString("    \n")

	// Título del reporte
	content.WriteString("    // Título\n")
	content.WriteString("    title [label=\"REPORTE DE BLOQUES UTILIZADOS\", fontsize=14, fontname=\"Arial Bold\", color=blue];\n")
	content.WriteString("    \n")

	usedBlocks := 0
	blocksPerRow := 6 // Mostrar 6 bloques por fila para mejor organización
	
	// Iterar por todos los bloques para encontrar los utilizados
	for i := int32(0); i < superblock.S_blocks_count; i++ {
		// Verificar si el bloque está en uso (bitmap)
		var bitmapByte byte
		if err := Utilities.ReadObject(file, &bitmapByte, int64(superblock.S_bm_block_start+i)); err != nil {
			continue // Error leyendo bitmap, continuar
		}
		
		// Si el bloque no está en uso, continuar
		if bitmapByte == 0 {
			continue
		}

		// Leer el bloque para determinar su tipo
		blockPos := int64(superblock.S_block_start + i*superblock.S_block_size)
		blockType, blockInfo := analyzeBlock(file, blockPos, i)

		usedBlocks++

		// Generar tabla para este bloque
		content.WriteString(fmt.Sprintf("    block_%d [label=<\n", i))
		content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"3\">\n")
		
		// Encabezado del bloque con color según el tipo
		bgColor := getBlockColor(blockType)
		
		content.WriteString(fmt.Sprintf("            <TR><TD COLSPAN=\"2\" BGCOLOR=\"%s\"><B>BLOQUE %d</B></TD></TR>\n", bgColor, i))
		content.WriteString(fmt.Sprintf("            <TR><TD><B>Tipo</B></TD><TD>%s</TD></TR>\n", blockType))
		content.WriteString(fmt.Sprintf("            <TR><TD><B>Info</B></TD><TD>%s</TD></TR>\n", blockInfo))
		
		content.WriteString("        </TABLE>\n")
		content.WriteString("    >];\n\n")
	}

	// Si no hay bloques utilizados
	if usedBlocks == 0 {
		content.WriteString("    no_blocks [label=<\n")
		content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
		content.WriteString("            <TR><TD BGCOLOR=\"#FFCDD2\"><B>NO HAY BLOQUES UTILIZADOS</B></TD></TR>\n")
		content.WriteString("            <TR><TD>El sistema de archivos no tiene bloques en uso</TD></TR>\n")
		content.WriteString("        </TABLE>\n")
		content.WriteString("    >];\n")
		content.WriteString("    title -> no_blocks;\n")
	} else {
		// Conectar título con el primer bloque
		content.WriteString("    title -> block_0 [style=invis];\n")
		
		// Organizar bloques en filas para mejor visualización
		content.WriteString("    // Organización visual en filas\n")
		rowCount := 0
		blocksInCurrentRow := 0
		blockList := make([]int32, 0)
		
		// Recolectar bloques utilizados
		for i := int32(0); i < superblock.S_blocks_count; i++ {
			var bitmapByte byte
			if err := Utilities.ReadObject(file, &bitmapByte, int64(superblock.S_bm_block_start+i)); err != nil {
				continue
			}
			if bitmapByte != 0 {
				blockList = append(blockList, i)
			}
		}
		
		// Crear filas de bloques
		for i, blockNum := range blockList {
			if blocksInCurrentRow == 0 {
				content.WriteString(fmt.Sprintf("    {rank=same; "))
			}
			
			content.WriteString(fmt.Sprintf("block_%d; ", blockNum))
			blocksInCurrentRow++
			
			if blocksInCurrentRow >= blocksPerRow || i == len(blockList)-1 {
				content.WriteString("}\n")
				blocksInCurrentRow = 0
				rowCount++
			}
		}
	}

	// Información del reporte
	content.WriteString("    \n")
	content.WriteString("    // Información del reporte\n")
	content.WriteString("    info [label=<\n")
	content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
	content.WriteString("            <TR><TD COLSPAN=\"2\" BGCOLOR=\"#E8F5E8\"><B>INFORMACIÓN DEL REPORTE</B></TD></TR>\n")
	content.WriteString(fmt.Sprintf("            <TR><TD><B>Total Bloques</B></TD><TD>%d</TD></TR>\n", superblock.S_blocks_count))
	content.WriteString(fmt.Sprintf("            <TR><TD><B>Bloques Utilizados</B></TD><TD>%d</TD></TR>\n", usedBlocks))
	content.WriteString(fmt.Sprintf("            <TR><TD><B>Bloques Libres</B></TD><TD>%d</TD></TR>\n", superblock.S_free_blocks_count))
	content.WriteString(fmt.Sprintf("            <TR><TD><B>Primer Bloque Libre</B></TD><TD>%d</TD></TR>\n", superblock.S_first_blo))
	content.WriteString("        </TABLE>\n")
	content.WriteString("    >];\n")

	// Cierre del archivo DOT
	content.WriteString("}\n")

	fmt.Printf("Bloques procesados: %d utilizados de %d totales\n", usedBlocks, superblock.S_blocks_count)
	return content.String()
}

// analyzeBlock analiza un bloque y determina su tipo y información básica
func analyzeBlock(file *os.File, blockPos int64, blockNum int32) (string, string) {
	// Leer los primeros bytes del bloque para determinar su tipo
	var block Structs.Fileblock
	if err := Utilities.ReadObject(file, &block, blockPos); err != nil {
		return "Error", "No se pudo leer"
	}

	// Primero analizar como archivo para ver si tiene contenido textual
	nonNullBytes := 0
	printableBytes := 0
	
	for i := 0; i < 64; i++ {
		if block.B_content[i] != 0 {
			nonNullBytes++
			if block.B_content[i] >= 32 && block.B_content[i] <= 126 {
				printableBytes++
			}
		}
	}

	// Si está vacío
	if nonNullBytes == 0 {
		return "Vacío", "Sin datos"
	}

	// Verificar si es un bloque de directorio (Folderblock)
	var folderBlock Structs.Folderblock
	if err := Utilities.ReadObject(file, &folderBlock, blockPos); err == nil {
		// Verificar si tiene estructura de directorio válida
		validEntries := 0
		validInodes := true
		
		for i := 0; i < 4; i++ {
			if folderBlock.B_content[i].B_inodo != -1 {
				validEntries++
				// Verificar que el inodo esté en un rango razonable (no sea demasiado grande)
				if folderBlock.B_content[i].B_inodo < 0 || folderBlock.B_content[i].B_inodo > 10000 {
					validInodes = false
					break
				}
			}
		}
		
		// Solo considerarlo directorio si tiene entradas válidas Y los inodos son razonables
		if validEntries > 0 && validInodes {
			// Verificar que al menos una entrada tenga un nombre válido
			hasValidName := false
			for i := 0; i < validEntries && i < 4; i++ {
				if folderBlock.B_content[i].B_inodo != -1 {
					name := cleanString(folderBlock.B_content[i].B_name[:])
					if name != "" && (name == "." || name == ".." || len(name) <= 12) {
						hasValidName = true
						break
					}
				}
			}
			
			if hasValidName {
				dirInfo := fmt.Sprintf("%d entradas", validEntries)
				// Mostrar el primer nombre válido
				for i := 0; i < 4; i++ {
					if folderBlock.B_content[i].B_inodo != -1 {
						fileName := cleanString(folderBlock.B_content[i].B_name[:])
						if fileName != "" {
							dirInfo += fmt.Sprintf(" ('%s'...)", fileName)
							break
						}
					}
				}
				return "Directorio", dirInfo
			}
		}
	}

	// Analizar como archivo de texto si la mayoría son caracteres imprimibles

	// Si la mayoría son caracteres imprimibles, probablemente es texto
	if printableBytes > nonNullBytes/2 {
		// Mostrar una preview del contenido (primeros caracteres)
		preview := ""
		for i := 0; i < 20 && i < len(block.B_content); i++ {
			if block.B_content[i] == 0 {
				break
			}
			if block.B_content[i] >= 32 && block.B_content[i] <= 126 {
				preview += string(block.B_content[i])
			} else {
				preview += "."
			}
		}
		if len(preview) > 15 {
			preview = preview[:15] + "..."
		}
		return "Archivo", fmt.Sprintf("'%s' (%d bytes)", preview, nonNullBytes)
	}

	// Verificar si podría ser un bloque de punteros (indirecto)
	// Los bloques de punteros contienen principalmente números (punteros a otros bloques)
	var pointerBlock Structs.Pointerblock
	if err := Utilities.ReadObject(file, &pointerBlock, blockPos); err == nil {
		validPointers := 0
		for i := 0; i < 16; i++ {
			if pointerBlock.B_pointers[i] >= 0 && pointerBlock.B_pointers[i] < 100000 { // Rango razonable
				validPointers++
			}
		}
		if validPointers > 0 {
			return "Punteros", fmt.Sprintf("%d punteros", validPointers)
		}
	}

	// Por defecto, considerarlo datos binarios
	return "Datos", fmt.Sprintf("%d bytes usados", nonNullBytes)
}

// getBlockColor retorna el color de fondo según el tipo de bloque
func getBlockColor(blockType string) string {
	switch blockType {
	case "Directorio":
		return "#FFF3E0" // Naranja claro
	case "Archivo":
		return "#E3F2FD" // Azul claro
	case "Punteros":
		return "#F3E5F5" // Púrpura claro
	case "Vacío":
		return "#F5F5F5" // Gris claro
	case "Error":
		return "#FFCDD2" // Rojo claro
	default:
		return "#E8F5E8" // Verde claro
	}
}

// GenerateBitmapInodeReport genera el reporte del bitmap de inodos en formato de texto
func GenerateBitmapInodeReport(userOutputPath string, partitionID string) error {
	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return fmt.Errorf("partición con ID %s no está montada", partitionID)
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// Leer el superblock para obtener información del bitmap
	var partition *Structs.Partition = nil
	var mbr Structs.MBR
	if err := Utilities.ReadObject(file, &mbr, 0); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Buscar la partición correspondiente
	if !mountedPartition.IsLogical {
		partition = &mbr.Partitions[mountedPartition.PartitionIndex]
	} else {
		return fmt.Errorf("particiones lógicas no soportadas aún para este reporte")
	}

	// Leer el superblock
	var superblock Structs.Superblock
	if err := Utilities.ReadObject(file, &superblock, int64(partition.Start)); err != nil {
		return fmt.Errorf("error leyendo superblock: %v", err)
	}

	// Crear el archivo de salida
	outputFile, err := os.Create(userOutputPath)
	if err != nil {
		return fmt.Errorf("error creando archivo de salida: %v", err)
	}
	defer outputFile.Close()

	// Escribir encabezado del reporte
	fmt.Fprintf(outputFile, "=== REPORTE BITMAP DE INODOS ===\n")
	fmt.Fprintf(outputFile, "Partición: %s\n", partitionID)
	fmt.Fprintf(outputFile, "Total de inodos: %d\n", superblock.S_inodes_count)
	fmt.Fprintf(outputFile, "Inodos libres: %d\n", superblock.S_free_inodes_count)
	fmt.Fprintf(outputFile, "Inodos utilizados: %d\n", superblock.S_inodes_count-superblock.S_free_inodes_count)
	fmt.Fprintf(outputFile, "====================================\n\n")

	// Leer y mostrar el bitmap de inodos

	bitCount := 0
	lineCount := 1

	for i := int32(0); i < superblock.S_inodes_count; i++ {
		var bitmapByte byte
		if err := Utilities.ReadObject(file, &bitmapByte, int64(superblock.S_bm_inode_start+i)); err != nil {
			return fmt.Errorf("error leyendo bitmap de inodos en posición %d: %v", i, err)
		}

		// Escribir el bit (0 o 1)
		fmt.Fprintf(outputFile, "%d", bitmapByte)

		bitCount++

		// Después de 20 bits, hacer nueva línea
		if bitCount%20 == 0 {
			fmt.Fprintf(outputFile, "\n")
			if i+1 < superblock.S_inodes_count {
				lineCount++
			}
		}
	}

	// Si la última línea no terminó, agregar salto de línea
	if bitCount%20 != 0 {
		fmt.Fprintf(outputFile, "\n")
	}

	fmt.Printf("✓ Reporte BM_INODE generado exitosamente\n")
	fmt.Printf("  - Archivo: %s\n", userOutputPath)
	fmt.Printf("  - Total inodos: %d\n", superblock.S_inodes_count)
	fmt.Printf("  - Inodos utilizados: %d\n", superblock.S_inodes_count-superblock.S_free_inodes_count)

	return nil
}

// GenerateBitmapBlockReport genera el reporte del bitmap de bloques en formato de texto
func GenerateBitmapBlockReport(userOutputPath string, partitionID string) error {
	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return fmt.Errorf("partición con ID %s no está montada", partitionID)
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// Leer el superblock para obtener información del bitmap
	var partition *Structs.Partition = nil
	var mbr Structs.MBR
	if err := Utilities.ReadObject(file, &mbr, 0); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Buscar la partición correspondiente
	if !mountedPartition.IsLogical {
		partition = &mbr.Partitions[mountedPartition.PartitionIndex]
	} else {
		return fmt.Errorf("particiones lógicas no soportadas aún para este reporte")
	}

	// Leer el superblock
	var superblock Structs.Superblock
	if err := Utilities.ReadObject(file, &superblock, int64(partition.Start)); err != nil {
		return fmt.Errorf("error leyendo superblock: %v", err)
	}

	// Crear el archivo de salida
	outputFile, err := os.Create(userOutputPath)
	if err != nil {
		return fmt.Errorf("error creando archivo de salida: %v", err)
	}
	defer outputFile.Close()

	// Escribir encabezado del reporte
	fmt.Fprintf(outputFile, "=== REPORTE BITMAP DE BLOQUES ===\n")
	fmt.Fprintf(outputFile, "Partición: %s\n", partitionID)
	fmt.Fprintf(outputFile, "Total de bloques: %d\n", superblock.S_blocks_count)
	fmt.Fprintf(outputFile, "Bloques libres: %d\n", superblock.S_free_blocks_count)
	fmt.Fprintf(outputFile, "Bloques utilizados: %d\n", superblock.S_blocks_count-superblock.S_free_blocks_count)
	fmt.Fprintf(outputFile, "====================================\n\n")

	// Leer y mostrar el bitmap de bloques
	fmt.Fprintf(outputFile, "Bitmap de Bloques:\n")

	bitCount := 0
	lineCount := 1

	for i := int32(0); i < superblock.S_blocks_count; i++ {
		var bitmapByte byte
		if err := Utilities.ReadObject(file, &bitmapByte, int64(superblock.S_bm_block_start+i)); err != nil {
			return fmt.Errorf("error leyendo bitmap de bloques en posición %d: %v", i, err)
		}

		// Escribir el bit (0 o 1)
		fmt.Fprintf(outputFile, "%d", bitmapByte)

		bitCount++

		// Después de 20 bits, hacer nueva línea
		if bitCount%20 == 0 {
			fmt.Fprintf(outputFile, "\n")
			if i+1 < superblock.S_blocks_count {
				lineCount++
			}
		}
	}

	// Si la última línea no terminó, agregar salto de línea
	if bitCount%20 != 0 {
		fmt.Fprintf(outputFile, "\n")
	}

	fmt.Printf("✓ Reporte BM_BLOCK generado exitosamente\n")
	fmt.Printf("  - Archivo: %s\n", userOutputPath)
	fmt.Printf("  - Total bloques: %d\n", superblock.S_blocks_count)
	fmt.Printf("  - Bloques utilizados: %d\n", superblock.S_blocks_count-superblock.S_free_blocks_count)

	return nil
}

// GenerateTreeReport genera el reporte del árbol completo del sistema EXT2
func GenerateTreeReport(userOutputPath string, partitionID string) error {
	// Obtener información de la partición montada
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return fmt.Errorf("partición con ID %s no está montada", partitionID)
	}

	// Abrir el archivo del disco
	file, err := Utilities.OpenFile(mountedPartition.Path)
	if err != nil {
		return fmt.Errorf("error abriendo disco: %v", err)
	}
	defer file.Close()

	// Leer el superblock
	var partition *Structs.Partition = nil
	var mbr Structs.MBR
	if err := Utilities.ReadObject(file, &mbr, 0); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Buscar la partición correspondiente
	if !mountedPartition.IsLogical {
		partition = &mbr.Partitions[mountedPartition.PartitionIndex]
	} else {
		return fmt.Errorf("particiones lógicas no soportadas aún para este reporte")
	}

	// Leer el superblock
	var superblock Structs.Superblock
	if err := Utilities.ReadObject(file, &superblock, int64(partition.Start)); err != nil {
		return fmt.Errorf("error leyendo superblock: %v", err)
	}

	// Generar el contenido DOT
	var content strings.Builder
	content.WriteString("digraph TreeReport {\n")
	content.WriteString("    graph [pad=\"0.5\", nodesep=\"0.8\", ranksep=\"1.2\"];\n")
	content.WriteString("    node [shape=plaintext]\n")
	content.WriteString("    rankdir=LR;\n\n")

	// Generar el árbol desde el inodo raíz
	connections := make([]string, 0)
	visitedInodes := make(map[int32]bool)
	visitedBlocks := make(map[int32]bool)

	// Comenzar desde el inodo raíz (inodo 0)
	generateTreeFromInode(file, &superblock, 0, "/", &content, &connections, visitedInodes, visitedBlocks)

	// Escribir las conexiones al final
	content.WriteString("\n    // Conexiones\n")
	for _, connection := range connections {
		content.WriteString("    " + connection + "\n")
	}

	content.WriteString("}\n")

	// Escribir archivo DOT
	dotFilePath := strings.TrimSuffix(userOutputPath, filepath.Ext(userOutputPath)) + ".dot"
	if err := os.WriteFile(dotFilePath, []byte(content.String()), 0644); err != nil {
		return fmt.Errorf("error escribiendo archivo DOT: %v", err)
	}

	// Generar imagen si Graphviz está instalado
	if checkGraphvizInstalled() {
		cmd := exec.Command("dot", "-Tjpg", dotFilePath, "-o", userOutputPath)
		if err := cmd.Run(); err != nil {
			fmt.Printf("Advertencia: No se pudo generar la imagen: %v\n", err)
			fmt.Printf("Asegúrate de tener Graphviz instalado (sudo apt install graphviz)\n")
		} else {
			fmt.Printf("✓ Imagen generada: %s\n", userOutputPath)
		}
	}

	fmt.Printf("✓ Reporte TREE generado exitosamente\n")
	fmt.Printf("  - Archivo DOT: %s\n", dotFilePath)
	fmt.Printf("  - Archivo imagen: %s\n", userOutputPath)

	return nil
}

// generateTreeFromInode genera recursivamente el árbol desde un inodo
func generateTreeFromInode(file *os.File, superblock *Structs.Superblock, inodeNum int32, name string, content *strings.Builder, connections *[]string, visitedInodes map[int32]bool, visitedBlocks map[int32]bool) {
	if visitedInodes[inodeNum] {
		return
	}
	visitedInodes[inodeNum] = true

	// Leer el inodo
	var inode Structs.Inode
	inodePos := int64(superblock.S_inode_start + inodeNum*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &inode, inodePos); err != nil {
		return
	}

	// Verificar si el inodo está en uso
	var bitmapByte byte
	if err := Utilities.ReadObject(file, &bitmapByte, int64(superblock.S_bm_inode_start+inodeNum)); err != nil {
		return
	}
	if bitmapByte == 0 {
		return // Inodo libre
	}

	inodeType := cleanString(inode.I_type[:])
	isDirectory := (inodeType == "0")
	
	// Limpiar el nombre para HTML
	cleanedName := strings.ReplaceAll(name, "&", "&amp;")
	cleanedName = strings.ReplaceAll(cleanedName, "<", "&lt;")
	cleanedName = strings.ReplaceAll(cleanedName, ">", "&gt;")

	// Generar nodo del inodo
	color := "lightyellow" // archivos
	if isDirectory {
		color = "lightgreen" // directorios
	}

	content.WriteString(fmt.Sprintf("    Inodo%d [\n", inodeNum))
	content.WriteString("        label=<\n")
	content.WriteString("            <table border=\"0\" cellborder=\"1\" cellspacing=\"0\" bgcolor=\"" + color + "\">\n")
	content.WriteString(fmt.Sprintf("                <tr><td colspan=\"2\" port='0'><b>Inodo %d (%s)</b></td></tr>\n", inodeNum, cleanedName))
	
	if isDirectory {
		content.WriteString("                <tr><td>Tipo</td><td>DIR</td></tr>\n")
	} else {
		content.WriteString("                <tr><td>Tipo</td><td>FILE</td></tr>\n")
	}

	// Mostrar bloques directos
	portNum := 1
	for i := 0; i < 12; i++ {
		if inode.I_block[i] != -1 {
			content.WriteString(fmt.Sprintf("                <tr><td>AD%d</td><td port='%d'>%d</td></tr>\n", i+1, portNum, inode.I_block[i]))
			portNum++
		} else if i < 2 { // Solo mostrar los primeros slots vacíos
			content.WriteString(fmt.Sprintf("                <tr><td>AD%d</td><td port='%d'>-</td></tr>\n", i+1, portNum))
			portNum++
		}
	}

	content.WriteString("            </table>\n")
	content.WriteString("        >\n")
	content.WriteString("    ];\n\n")

	// Procesar bloques del inodo
	portNum = 1
	for i := 0; i < 12; i++ {
		if inode.I_block[i] != -1 {
			blockNum := inode.I_block[i]
			*connections = append(*connections, fmt.Sprintf("Inodo%d:%d -> Bloque%d:0;", inodeNum, portNum, blockNum))
			
			if isDirectory {
				generateTreeFromDirectoryBlock(file, superblock, blockNum, content, connections, visitedInodes, visitedBlocks)
			} else {
				generateTreeFromFileBlock(file, superblock, blockNum, content, visitedBlocks)
			}
			portNum++
		} else if i < 2 {
			portNum++
		}
	}
}

// generateTreeFromDirectoryBlock procesa un bloque de directorio
func generateTreeFromDirectoryBlock(file *os.File, superblock *Structs.Superblock, blockNum int32, content *strings.Builder, connections *[]string, visitedInodes map[int32]bool, visitedBlocks map[int32]bool) {
	if visitedBlocks[blockNum] {
		return
	}
	visitedBlocks[blockNum] = true

	// Verificar si el bloque está en uso
	var bitmapByte byte
	if err := Utilities.ReadObject(file, &bitmapByte, int64(superblock.S_bm_block_start+blockNum)); err != nil {
		return
	}
	if bitmapByte == 0 {
		return // Bloque libre
	}

	// Leer bloque de directorio
	var folderBlock Structs.Folderblock
	blockPos := int64(superblock.S_block_start + blockNum*superblock.S_block_size)
	if err := Utilities.ReadObject(file, &folderBlock, blockPos); err != nil {
		return
	}

	// Generar nodo del bloque
	content.WriteString(fmt.Sprintf("    Bloque%d [\n", blockNum))
	content.WriteString("        label=<\n")
	content.WriteString("            <table border=\"0\" cellborder=\"1\" cellspacing=\"0\" bgcolor=\"lightblue\">\n")
	content.WriteString(fmt.Sprintf("                <tr><td colspan=\"2\" port='0'><b>Bloque %d</b></td></tr>\n", blockNum))

	// Primero recopilar todas las entradas válidas
	entries := make([]struct {
		name    string
		inodeNum int32
	}, 0)

	for i := 0; i < 4; i++ {
		if folderBlock.B_content[i].B_inodo != -1 {
			entryName := cleanString(folderBlock.B_content[i].B_name[:])
			if entryName != "" && entryName != "." && entryName != ".." {
				// Limpiar nombre para HTML
				cleanedEntryName := strings.ReplaceAll(entryName, "&", "&amp;")
				cleanedEntryName = strings.ReplaceAll(cleanedEntryName, "<", "&lt;")
				cleanedEntryName = strings.ReplaceAll(cleanedEntryName, ">", "&gt;")
				
				entries = append(entries, struct {
					name    string
					inodeNum int32
				}{cleanedEntryName, folderBlock.B_content[i].B_inodo})
			}
		}
	}

	// Escribir las entradas en la tabla
	portNum := 1
	for _, entry := range entries {
		content.WriteString(fmt.Sprintf("                <tr><td>%s</td><td port='%d'>%d</td></tr>\n", entry.name, portNum, entry.inodeNum))
		portNum++
	}

	content.WriteString("            </table>\n")
	content.WriteString("        >\n")
	content.WriteString("    ];\n\n")

	// Crear conexiones y procesar inodos hijos
	portNum = 1
	for _, entry := range entries {
		// Crear conexión al inodo hijo
		*connections = append(*connections, fmt.Sprintf("Bloque%d:%d -> Inodo%d:0;", blockNum, portNum, entry.inodeNum))
		
		// Procesar recursivamente el inodo hijo
		generateTreeFromInode(file, superblock, entry.inodeNum, entry.name, content, connections, visitedInodes, visitedBlocks)
		
		portNum++
	}
}

// generateTreeFromFileBlock procesa un bloque de archivo
func generateTreeFromFileBlock(file *os.File, superblock *Structs.Superblock, blockNum int32, content *strings.Builder, visitedBlocks map[int32]bool) {
	if visitedBlocks[blockNum] {
		return
	}
	visitedBlocks[blockNum] = true

	// Verificar si el bloque está en uso
	var bitmapByte byte
	if err := Utilities.ReadObject(file, &bitmapByte, int64(superblock.S_bm_block_start+blockNum)); err != nil {
		return
	}
	if bitmapByte == 0 {
		return // Bloque libre
	}

	// Leer bloque de archivo
	var fileBlock Structs.Fileblock
	blockPos := int64(superblock.S_block_start + blockNum*superblock.S_block_size)
	if err := Utilities.ReadObject(file, &fileBlock, blockPos); err != nil {
		return
	}

	// Obtener preview del contenido
	var preview string
	for i := 0; i < 64; i++ {
		if fileBlock.B_content[i] == 0 {
			break
		}
		if fileBlock.B_content[i] >= 32 && fileBlock.B_content[i] <= 126 {
			preview += string(fileBlock.B_content[i])
		} else {
			preview += "."
		}
	}

	// Limitar longitud del preview y escapar para HTML
	if len(preview) > 30 {
		preview = preview[:30] + "..."
	}
	preview = strings.ReplaceAll(preview, "&", "&amp;")
	preview = strings.ReplaceAll(preview, "<", "&lt;")
	preview = strings.ReplaceAll(preview, ">", "&gt;")
	preview = strings.ReplaceAll(preview, "\n", "<br/>")

	// Generar nodo del bloque
	content.WriteString(fmt.Sprintf("    Bloque%d [\n", blockNum))
	content.WriteString("        label=<\n")
	content.WriteString("            <table border=\"0\" cellborder=\"1\" cellspacing=\"0\" bgcolor=\"lightcoral\">\n")
	content.WriteString(fmt.Sprintf("                <tr><td colspan=\"2\" port='0'><b>Bloque %d</b></td></tr>\n", blockNum))
	content.WriteString(fmt.Sprintf("                <tr><td colspan=\"2\">%s</td></tr>\n", preview))
	content.WriteString("            </table>\n")
	content.WriteString("        >\n")
	content.WriteString("    ];\n\n")
}

// checkGraphvizInstalled verifica si Graphviz está instalado en el sistema
func checkGraphvizInstalled() bool {
	cmd := exec.Command("dot", "-V")
	err := cmd.Run()
	return err == nil
}

// ============================================================================
// REPORTE SB (SUPERBLOCK)
// ============================================================================

// GenerateSuperblockReport genera el reporte del superbloque en formato DOT e imagen
func GenerateSuperblockReport(userOutputPath string, partitionID string) error {
	fmt.Println("=== GENERANDO REPORTE SUPERBLOCK ===")
	fmt.Printf("Ruta de salida especificada: %s\n", userOutputPath)
	fmt.Printf("ID de partición: %s\n", partitionID)
	
	// Buscar la partición montada para obtener la ruta del disco
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return fmt.Errorf("la partición con ID '%s' no está montada", partitionID)
	}

	diskPath := mountedPartition.Path
	fmt.Printf("Ruta del disco: %s\n", diskPath)

	// Abrir archivo del disco
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("error abriendo archivo del disco: %v", err)
	}
	defer file.Close()

	// Leer el superblock para obtener la información
	var tempMBR Structs.MBR
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Obtener la partición correcta
	var partition *Structs.Partition = nil
	if !mountedPartition.IsLogical {
		partition = &tempMBR.Partitions[mountedPartition.PartitionIndex]
	} else {
		// Para partición lógica, crear una partición temporal
		var tempEBR Structs.EBR
		if err := Utilities.ReadObject(file, &tempEBR, int64(mountedPartition.EBRPosition)); err != nil {
			return fmt.Errorf("error leyendo EBR: %v", err)
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
		return fmt.Errorf("error leyendo superblock: %v", err)
	}

	// Usar la ruta exacta especificada por el usuario
	finalDotPath, finalImagePath := processUserPath(userOutputPath)
	fmt.Printf("Archivo DOT: %s\n", finalDotPath)
	fmt.Printf("Archivo imagen: %s\n", finalImagePath)

	// Crear directorio de salida si no existe
	if err := createOutputDirectory(finalDotPath); err != nil {
		return fmt.Errorf("error creando directorio de salida: %v", err)
	}

	// Generar contenido del reporte en formato DOT
	dotContent := generateSuperblockDotContent(&superblock)

	// Escribir archivo DOT
	if err := writeReportFile(finalDotPath, dotContent); err != nil {
		return fmt.Errorf("error escribiendo archivo DOT: %v", err)
	}

	// Generar imagen usando Graphviz
	if err := generateGraphvizImage(finalDotPath, finalImagePath); err != nil {
		fmt.Printf("Advertencia: No se pudo generar la imagen: %v\n", err)
		fmt.Println("Asegúrate de tener Graphviz instalado (sudo apt install graphviz)")
	} else {
		fmt.Printf("✓ Imagen generada: %s\n", finalImagePath)
	}

	fmt.Printf("✓ Reporte SB generado exitosamente\n")
	fmt.Printf("  - Archivo DOT: %s\n", finalDotPath)
	fmt.Printf("  - Archivo imagen: %s\n", finalImagePath)
	return nil
}

// generateSuperblockDotContent genera el contenido del reporte SB en formato DOT
func generateSuperblockDotContent(superblock *Structs.Superblock) string {
	var content strings.Builder

	// Encabezado del archivo DOT
	content.WriteString("digraph superblock_report {\n")
	content.WriteString("    // Configuración general\n")
	content.WriteString("    node [shape=plaintext, fontname=\"Arial\", fontsize=10];\n")
	content.WriteString("    rankdir=TB;\n")
	content.WriteString("    \n")

	// Título del reporte
	content.WriteString("    // Título\n")
	content.WriteString("    title [label=\"REPORTE DE SUPERBLOQUE\", fontsize=16, fontname=\"Arial Bold\", color=blue];\n")
	content.WriteString("    \n")

	// Tabla principal del superbloque
	content.WriteString("    superblock [label=<\n")
	content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
	
	// Encabezado de la tabla
	content.WriteString("            <TR><TD COLSPAN=\"2\" BGCOLOR=\"#2196F3\"><FONT COLOR=\"white\"><B>INFORMACIÓN DEL SUPERBLOQUE</B></FONT></TD></TR>\n")
	
	// Información del sistema de archivos
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#E3F2FD\"><B>Tipo de Sistema de Archivos</B></TD><TD>%d</TD></TR>\n", superblock.S_filesystem_type))
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#E3F2FD\"><B>Número de Inodos</B></TD><TD>%d</TD></TR>\n", superblock.S_inodes_count))
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#E3F2FD\"><B>Número de Bloques</B></TD><TD>%d</TD></TR>\n", superblock.S_blocks_count))
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#E3F2FD\"><B>Bloques Libres</B></TD><TD>%d</TD></TR>\n", superblock.S_free_blocks_count))
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#E3F2FD\"><B>Inodos Libres</B></TD><TD>%d</TD></TR>\n", superblock.S_free_inodes_count))
	
	// Fechas (limpiar caracteres nulos)
	mtime := cleanString(superblock.S_mtime[:])
	umtime := cleanString(superblock.S_umtime[:])
	if mtime == "" {
		mtime = "No disponible"
	}
	if umtime == "" {
		umtime = "No disponible"
	}
	
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#FFF3E0\"><B>Último Montaje</B></TD><TD>%s</TD></TR>\n", mtime))
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#FFF3E0\"><B>Último Desmontaje</B></TD><TD>%s</TD></TR>\n", umtime))
	
	// Información de montaje y sistema
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#F3E5F5\"><B>Contador de Montajes</B></TD><TD>%d</TD></TR>\n", superblock.S_mnt_count))
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#F3E5F5\"><B>Número Mágico</B></TD><TD>%d (0x%X)</TD></TR>\n", superblock.S_magic, superblock.S_magic))
	
	// Tamaños
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#E8F5E8\"><B>Tamaño de Inodo</B></TD><TD>%d bytes</TD></TR>\n", superblock.S_inode_size))
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#E8F5E8\"><B>Tamaño de Bloque</B></TD><TD>%d bytes</TD></TR>\n", superblock.S_block_size))
	
	// Ubicaciones y punteros
	content.WriteString("            <TR><TD COLSPAN=\"2\" BGCOLOR=\"#FF9800\"><FONT COLOR=\"white\"><B>UBICACIONES EN EL DISCO</B></FONT></TD></TR>\n")
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#FFF3E0\"><B>Primer Inodo Libre</B></TD><TD>%d</TD></TR>\n", superblock.S_fist_ino))
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#FFF3E0\"><B>Primer Bloque Libre</B></TD><TD>%d</TD></TR>\n", superblock.S_first_blo))
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#FFF3E0\"><B>Inicio Bitmap Inodos</B></TD><TD>%d</TD></TR>\n", superblock.S_bm_inode_start))
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#FFF3E0\"><B>Inicio Bitmap Bloques</B></TD><TD>%d</TD></TR>\n", superblock.S_bm_block_start))
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#FFF3E0\"><B>Inicio Tabla Inodos</B></TD><TD>%d</TD></TR>\n", superblock.S_inode_start))
	content.WriteString(fmt.Sprintf("            <TR><TD BGCOLOR=\"#FFF3E0\"><B>Inicio Bloques de Datos</B></TD><TD>%d</TD></TR>\n", superblock.S_block_start))
	
	content.WriteString("        </TABLE>\n")
	content.WriteString("    >];\n")

	// Conectar elementos visualmente
	content.WriteString("    \n")
	content.WriteString("    // Conexiones para organización visual\n")
	content.WriteString("    title -> superblock [style=invis];\n")
	content.WriteString("    superblock -> stats [style=invis];\n")

	// Cierre del archivo DOT
	content.WriteString("}\n")

	fmt.Printf("Superbloque procesado:\n")
	fmt.Printf("  - Inodos: %d total, %d libres\n", superblock.S_inodes_count, superblock.S_free_inodes_count)
	fmt.Printf("  - Bloques: %d total, %d libres\n", superblock.S_blocks_count, superblock.S_free_blocks_count)

	return content.String()
}

// ============================================================================
// REPORTE FILE
// ============================================================================

// GenerateFileReport genera el reporte de un archivo específico mostrando su contenido
func GenerateFileReport(userOutputPath string, partitionID string, filePath string) error {
	fmt.Println("=== GENERANDO REPORTE FILE ===")
	fmt.Printf("Ruta de salida especificada: %s\n", userOutputPath)
	fmt.Printf("ID de partición: %s\n", partitionID)
	fmt.Printf("Archivo a mostrar: %s\n", filePath)
	
	// Buscar la partición montada para obtener la ruta del disco
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return fmt.Errorf("la partición con ID '%s' no está montada", partitionID)
	}

	diskPath := mountedPartition.Path
	fmt.Printf("Ruta del disco: %s\n", diskPath)

	// Abrir archivo del disco
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("error abriendo archivo del disco: %v", err)
	}
	defer file.Close()

	// Leer el superblock para obtener la información del sistema de archivos
	var tempMBR Structs.MBR
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Obtener la partición correcta
	var partition *Structs.Partition = nil
	if !mountedPartition.IsLogical {
		partition = &tempMBR.Partitions[mountedPartition.PartitionIndex]
	} else {
		return fmt.Errorf("particiones lógicas no soportadas aún para este reporte")
	}

	// Leer el superblock
	var superblock Structs.Superblock
	if err := Utilities.ReadObject(file, &superblock, int64(partition.Start)); err != nil {
		return fmt.Errorf("error leyendo superblock: %v", err)
	}

	// Buscar el archivo en el sistema de archivos
	fileContent, fileName, err := findFileInFilesystem(file, &superblock, filePath)
	if err != nil {
		return fmt.Errorf("error buscando archivo '%s': %v", filePath, err)
	}

	// Crear el archivo de salida
	outputFile, err := os.Create(userOutputPath)
	if err != nil {
		return fmt.Errorf("error creando archivo de salida: %v", err)
	}
	defer outputFile.Close()

	// Escribir el reporte
	fmt.Fprintf(outputFile, "=== REPORTE DE ARCHIVO ===\n")
	fmt.Fprintf(outputFile, "Nombre del archivo: %s\n", fileName)
	fmt.Fprintf(outputFile, "============================\n\n")
	fmt.Fprintf(outputFile, "CONTENIDO DEL ARCHIVO:\n")
	fmt.Fprintf(outputFile, "======================\n")
	fmt.Fprintf(outputFile, "%s", fileContent)

	fmt.Printf("✓ Reporte FILE generado exitosamente\n")
	fmt.Printf("  - Archivo: %s\n", userOutputPath)
	fmt.Printf("  - Archivo encontrado: %s\n", fileName)
	fmt.Printf("  - Tamaño del contenido: %d bytes\n", len(fileContent))

	return nil
}

// findFileInFilesystem busca un archivo en el sistema y retorna su contenido
func findFileInFilesystem(file *os.File, superblock *Structs.Superblock, targetPath string) (string, string, error) {
	// Limpiar y dividir la ruta
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return "", "", fmt.Errorf("ruta de archivo vacía")
	}

	// Si la ruta no empieza con /, agregarla
	if !strings.HasPrefix(targetPath, "/") {
		targetPath = "/" + targetPath
	}

	// Dividir la ruta en componentes
	pathComponents := strings.Split(strings.Trim(targetPath, "/"), "/")
	
	// Si es la ruta raíz, no hay archivo que mostrar
	if len(pathComponents) == 1 && pathComponents[0] == "" {
		return "", "", fmt.Errorf("no se puede mostrar el contenido del directorio raíz")
	}

	fmt.Printf("Buscando archivo: %s\n", targetPath)
	fmt.Printf("Componentes de ruta: %v\n", pathComponents)

	// Comenzar la búsqueda desde el inodo raíz (inodo 0)
	currentInodeNum := int32(0)
	var currentPath strings.Builder
	currentPath.WriteString("/")

	// Navegar por cada componente de la ruta
	for i, component := range pathComponents {
		if component == "" {
			continue
		}

		fmt.Printf("Buscando componente: '%s' en inodo %d\n", component, currentInodeNum)

		// Leer el inodo actual
		var currentInode Structs.Inode
		inodePos := int64(superblock.S_inode_start + currentInodeNum*superblock.S_inode_size)
		if err := Utilities.ReadObject(file, &currentInode, inodePos); err != nil {
			return "", "", fmt.Errorf("error leyendo inodo %d: %v", currentInodeNum, err)
		}

		// Verificar que el inodo esté en uso
		var bitmapByte byte
		if err := Utilities.ReadObject(file, &bitmapByte, int64(superblock.S_bm_inode_start+currentInodeNum)); err != nil {
			return "", "", fmt.Errorf("error leyendo bitmap de inodo %d: %v", currentInodeNum, err)
		}
		if bitmapByte == 0 {
			return "", "", fmt.Errorf("inodo %d no está en uso", currentInodeNum)
		}

		inodeType := cleanString(currentInode.I_type[:])
		isDirectory := (inodeType == "0")
		


		// Si no es el último componente, el inodo actual debe ser un directorio
		if i < len(pathComponents)-1 {
			if !isDirectory {
				return "", "", fmt.Errorf("'%s' no es un directorio", currentPath.String()+component)
			}

			// Buscar el siguiente componente en este directorio
			nextInodeNum, found, err := findInodeInDirectory(file, &currentInode, superblock, component)
			if err != nil {
				return "", "", fmt.Errorf("error buscando en directorio: %v", err)
			}
			if !found {
				return "", "", fmt.Errorf("no se encontró '%s' en '%s'", component, currentPath.String())
			}

			// Actualizar para el siguiente nivel
			currentInodeNum = nextInodeNum
			currentPath.WriteString(component)
			currentPath.WriteString("/")
		} else {
			// Es el último componente - buscar el archivo en el directorio actual
			if !isDirectory {
				return "", "", fmt.Errorf("error: inodo padre %d no es un directorio", currentInodeNum)
			}

			// Buscar el archivo en este directorio
			targetInodeNum, found, err := findInodeInDirectory(file, &currentInode, superblock, component)
			if err != nil {
				return "", "", fmt.Errorf("error buscando archivo en directorio: %v", err)
			}
			if !found {
				return "", "", fmt.Errorf("no se encontró el archivo '%s' en '%s'", component, currentPath.String())
			}

			// Leer el inodo del archivo encontrado
			var targetInode Structs.Inode
			targetInodePos := int64(superblock.S_inode_start + targetInodeNum*superblock.S_inode_size)
			if err := Utilities.ReadObject(file, &targetInode, targetInodePos); err != nil {
				return "", "", fmt.Errorf("error leyendo inodo del archivo %d: %v", targetInodeNum, err)
			}

			targetInodeType := cleanString(targetInode.I_type[:])
			targetIsDirectory := (targetInodeType == "0")
			


			if targetIsDirectory {
				return "", "", fmt.Errorf("'%s' es un directorio, no un archivo", targetPath)
			}

			// Es un archivo, leer su contenido
			fileContent, err := readFileContent(file, &targetInode, superblock)
			if err != nil {
				return "", "", fmt.Errorf("error leyendo contenido del archivo: %v", err)
			}

			return fileContent, currentPath.String() + component, nil
		}
	}

	return "", "", fmt.Errorf("no se pudo resolver la ruta")
}

// readFileContent lee el contenido completo de un archivo desde sus bloques
func readFileContent(file *os.File, inode *Structs.Inode, superblock *Structs.Superblock) (string, error) {
	var content strings.Builder

	// Leer bloques directos (0-11)
	for i := 0; i < 12; i++ {
		if inode.I_block[i] == -1 {
			continue // Bloque no asignado
		}

		// Leer el bloque
		var fileBlock Structs.Fileblock
		blockPos := int64(superblock.S_block_start + inode.I_block[i]*superblock.S_block_size)
		if err := Utilities.ReadObject(file, &fileBlock, blockPos); err != nil {
			return "", fmt.Errorf("error leyendo bloque %d: %v", inode.I_block[i], err)
		}

		// Agregar el contenido del bloque
		for j := 0; j < 64; j++ {
			if fileBlock.B_content[j] == 0 {
				break
			}
			content.WriteByte(fileBlock.B_content[j])
		}
	}


	return content.String(), nil
}

// findInodeInDirectory busca una entrada específica en un directorio
func findInodeInDirectory(file *os.File, dirInode *Structs.Inode, superblock *Structs.Superblock, targetName string) (int32, bool, error) {
	// Buscar en los bloques directos del directorio
	for i := 0; i < 12; i++ {
		if dirInode.I_block[i] == -1 {
			continue // Bloque no asignado
		}

		// Leer el bloque de directorio
		var folderBlock Structs.Folderblock
		blockPos := int64(superblock.S_block_start + dirInode.I_block[i]*superblock.S_block_size)
		if err := Utilities.ReadObject(file, &folderBlock, blockPos); err != nil {
			return -1, false, fmt.Errorf("error leyendo bloque de directorio %d: %v", dirInode.I_block[i], err)
		}

		// Buscar en las entradas del bloque
		for j := 0; j < 4; j++ {
			if folderBlock.B_content[j].B_inodo == -1 {
				continue // Entrada vacía
			}

			entryName := cleanString(folderBlock.B_content[j].B_name[:])
			if entryName == targetName {
				return folderBlock.B_content[j].B_inodo, true, nil
			}
		}
	}

	return -1, false, nil // No encontrado
}

// ============================================================================
// REPORTE LS
// ============================================================================

// GenerateListReport genera el reporte ls que muestra información detallada de archivos y directorios
func GenerateListReport(userOutputPath string, partitionID string, dirPath string) error {
	fmt.Println("=== GENERANDO REPORTE LS ===")
	fmt.Printf("Ruta de salida especificada: %s\n", userOutputPath)
	fmt.Printf("ID de partición: %s\n", partitionID)
	fmt.Printf("Directorio a listar: %s\n", dirPath)
	
	// Buscar la partición montada para obtener la ruta del disco
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return fmt.Errorf("la partición con ID '%s' no está montada", partitionID)
	}

	diskPath := mountedPartition.Path
	fmt.Printf("Ruta del disco: %s\n", diskPath)

	// Abrir archivo del disco
	file, err := Utilities.OpenFile(diskPath)
	if err != nil {
		return fmt.Errorf("error abriendo archivo del disco: %v", err)
	}
	defer file.Close()

	// Leer el superblock para obtener la información del sistema de archivos
	var tempMBR Structs.MBR
	if err := Utilities.ReadObject(file, &tempMBR, 0); err != nil {
		return fmt.Errorf("error leyendo MBR: %v", err)
	}

	// Obtener la partición correcta
	var partition *Structs.Partition = nil
	if !mountedPartition.IsLogical {
		partition = &tempMBR.Partitions[mountedPartition.PartitionIndex]
	} else {
		return fmt.Errorf("particiones lógicas no soportadas aún para este reporte")
	}

	// Leer el superblock
	var superblock Structs.Superblock
	if err := Utilities.ReadObject(file, &superblock, int64(partition.Start)); err != nil {
		return fmt.Errorf("error leyendo superblock: %v", err)
	}

	// Buscar el directorio en el sistema de archivos
	directoryEntries, err := listDirectoryContents(file, &superblock, dirPath)
	if err != nil {
		return fmt.Errorf("error listando directorio '%s': %v", dirPath, err)
	}

	// Usar la ruta exacta especificada por el usuario
	finalDotPath, finalImagePath := processUserPath(userOutputPath)
	fmt.Printf("Archivo DOT: %s\n", finalDotPath)
	fmt.Printf("Archivo imagen: %s\n", finalImagePath)

	// Crear directorio de salida si no existe
	if err := createOutputDirectory(finalDotPath); err != nil {
		return fmt.Errorf("error creando directorio de salida: %v", err)
	}

	// Generar contenido del reporte en formato DOT
	dotContent := generateLsDotContent(directoryEntries, dirPath, partitionID)

	// Escribir archivo DOT
	if err := writeReportFile(finalDotPath, dotContent); err != nil {
		return fmt.Errorf("error escribiendo archivo DOT: %v", err)
	}

	// Generar imagen usando Graphviz
	if err := generateGraphvizImage(finalDotPath, finalImagePath); err != nil {
		fmt.Printf("Advertencia: No se pudo generar la imagen: %v\n", err)
		fmt.Println("Asegúrate de tener Graphviz instalado (sudo apt install graphviz)")
	} else {
		fmt.Printf("✓ Imagen generada: %s\n", finalImagePath)
	}

	fmt.Printf("✓ Reporte LS generado exitosamente\n")
	fmt.Printf("  - Archivo DOT: %s\n", finalDotPath)
	fmt.Printf("  - Archivo imagen: %s\n", finalImagePath)
	fmt.Printf("  - Entradas listadas: %d\n", len(directoryEntries))

	return nil
}

// DirectoryEntry representa una entrada de directorio con toda su información
type DirectoryEntry struct {
	Name           string
	Type           string // "FILE" o "DIR"
	Permissions    string
	Owner          string
	Group          string
	Size           int32
	CreationTime   string
	ModificationTime string
	AccessTime     string
	InodeNumber    int32
}

// listDirectoryContents lista el contenido de un directorio específico
func listDirectoryContents(file *os.File, superblock *Structs.Superblock, targetPath string) ([]DirectoryEntry, error) {
	// Limpiar y dividir la ruta
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		targetPath = "/"
	}

	// Si la ruta no empieza con /, agregarla (ruta absoluta desde raíz)
	if !strings.HasPrefix(targetPath, "/") {
		targetPath = "/" + targetPath
	}

	fmt.Printf("Listando directorio: %s\n", targetPath)

	// Encontrar el inodo del directorio objetivo
	dirInodeNum, err := findDirectoryInode(file, superblock, targetPath)
	if err != nil {
		return nil, fmt.Errorf("error encontrando directorio: %v", err)
	}

	// Leer el inodo del directorio
	var dirInode Structs.Inode
	inodePos := int64(superblock.S_inode_start + dirInodeNum*superblock.S_inode_size)
	if err := Utilities.ReadObject(file, &dirInode, inodePos); err != nil {
		return nil, fmt.Errorf("error leyendo inodo del directorio %d: %v", dirInodeNum, err)
	}

	// Verificar que es un directorio
	inodeType := cleanString(dirInode.I_type[:])
	if inodeType != "0" {
		return nil, fmt.Errorf("la ruta especificada no es un directorio")
	}

	var entries []DirectoryEntry

	// Leer todos los bloques del directorio
	for i := 0; i < 12; i++ {
		if dirInode.I_block[i] == -1 {
			continue // Bloque no asignado
		}

		// Leer el bloque de directorio
		var folderBlock Structs.Folderblock
		blockPos := int64(superblock.S_block_start + dirInode.I_block[i]*superblock.S_block_size)
		if err := Utilities.ReadObject(file, &folderBlock, blockPos); err != nil {
			continue // Error leyendo bloque, continuar
		}

		// Procesar cada entrada del bloque
		for j := 0; j < 4; j++ {
			if folderBlock.B_content[j].B_inodo == -1 {
				continue // Entrada vacía
			}

			entryName := cleanString(folderBlock.B_content[j].B_name[:])
			if entryName == "" {
				continue // Nombre vacío
			}

			// Leer el inodo de la entrada
			entryInodeNum := folderBlock.B_content[j].B_inodo
			var entryInode Structs.Inode
			entryInodePos := int64(superblock.S_inode_start + entryInodeNum*superblock.S_inode_size)
			if err := Utilities.ReadObject(file, &entryInode, entryInodePos); err != nil {
				continue // Error leyendo inodo, continuar
			}

			// Crear la entrada del directorio
			entry := DirectoryEntry{
				Name:             entryName,
				InodeNumber:      entryInodeNum,
				Size:             entryInode.I_size,
				CreationTime:     cleanString(entryInode.I_ctime[:]),
				ModificationTime: cleanString(entryInode.I_mtime[:]),
				AccessTime:       cleanString(entryInode.I_atime[:]),
				Permissions:      cleanString(entryInode.I_perm[:]),
			}

			// Determinar tipo
			entryType := cleanString(entryInode.I_type[:])
			if entryType == "0" {
				entry.Type = "DIR"
			} else {
				entry.Type = "FILE"
			}

			// Obtener información del propietario y grupo
			entry.Owner = fmt.Sprintf("%d", entryInode.I_uid)
			entry.Group = fmt.Sprintf("%d", entryInode.I_gid)

			entries = append(entries, entry)
		}
	}

	fmt.Printf("Encontradas %d entradas en el directorio\n", len(entries))
	return entries, nil
}

// findDirectoryInode encuentra el inodo de un directorio dado su ruta
func findDirectoryInode(file *os.File, superblock *Structs.Superblock, targetPath string) (int32, error) {
	// Si es la raíz, retornar inodo 0
	if targetPath == "/" {
		return 0, nil
	}

	// Dividir la ruta en componentes
	pathComponents := strings.Split(strings.Trim(targetPath, "/"), "/")
	
	// Comenzar desde el inodo raíz
	currentInodeNum := int32(0)

	// Navegar por cada componente
	for _, component := range pathComponents {
		if component == "" {
			continue
		}

		// Leer el inodo actual
		var currentInode Structs.Inode
		inodePos := int64(superblock.S_inode_start + currentInodeNum*superblock.S_inode_size)
		if err := Utilities.ReadObject(file, &currentInode, inodePos); err != nil {
			return -1, fmt.Errorf("error leyendo inodo %d: %v", currentInodeNum, err)
		}

		// Verificar que es un directorio
		inodeType := cleanString(currentInode.I_type[:])
		if inodeType != "0" {
			return -1, fmt.Errorf("componente de ruta '%s' no es un directorio", component)
		}

		// Buscar el siguiente componente
		nextInodeNum, found, err := findInodeInDirectory(file, &currentInode, superblock, component)
		if err != nil {
			return -1, fmt.Errorf("error buscando componente '%s': %v", component, err)
		}
		if !found {
			return -1, fmt.Errorf("componente '%s' no encontrado", component)
		}

		currentInodeNum = nextInodeNum
	}

	return currentInodeNum, nil
}

// generateLsDotContent genera el contenido DOT para el reporte LS
func generateLsDotContent(entries []DirectoryEntry, dirPath string, partitionID string) string {
	var content strings.Builder

	// Encabezado del archivo DOT
	content.WriteString("digraph ls_report {\n")
	content.WriteString("    // Configuración general\n")
	content.WriteString("    node [shape=plaintext, fontname=\"Arial\", fontsize=9];\n")
	content.WriteString("    rankdir=TB;\n")
	content.WriteString("    \n")

	if len(entries) == 0 {
		// Directorio vacío
		content.WriteString("    empty [label=<\n")
		content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
		content.WriteString("            <TR><TD BGCOLOR=\"#FFCDD2\"><B>DIRECTORIO VACÍO</B></TD></TR>\n")
		content.WriteString("            <TR><TD>No se encontraron archivos o subdirectorios</TD></TR>\n")
		content.WriteString("        </TABLE>\n")
		content.WriteString("    >];\n")
		content.WriteString("    title -> info -> empty [style=invis];\n")
	} else {
		// Tabla principal con las entradas
		content.WriteString("    entries [label=<\n")
		content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"3\">\n")
		
		// Encabezado de la tabla
		content.WriteString("            <TR BGCOLOR=\"#4CAF50\">\n")
		content.WriteString("                <TD><FONT COLOR=\"black\"><B>Nombre</B></FONT></TD>\n")
		content.WriteString("                <TD><FONT COLOR=\"black\"><B>Tipo</B></FONT></TD>\n")
		content.WriteString("                <TD><FONT COLOR=\"black\"><B>Permisos</B></FONT></TD>\n")
		content.WriteString("                <TD><FONT COLOR=\"black\"><B>Propietario</B></FONT></TD>\n")
		content.WriteString("                <TD><FONT COLOR=\"black\"><B>Grupo</B></FONT></TD>\n")
		content.WriteString("                <TD><FONT COLOR=\"black\"><B>Tamaño</B></FONT></TD>\n")
		content.WriteString("                <TD><FONT COLOR=\"black\"><B>Creación</B></FONT></TD>\n")
		content.WriteString("                <TD><FONT COLOR=\"black\"><B>Modificación</B></FONT></TD>\n")
		content.WriteString("                <TD><FONT COLOR=\"black\"><B>Inodo</B></FONT></TD>\n")
		content.WriteString("            </TR>\n")

		// Entries data
		for _, entry := range entries {
			bgColor := "#E3F2FD" 
			if entry.Type == "DIR" {
				bgColor = "#FFF3E0" // Naranja claro para directorios
			}

			content.WriteString(fmt.Sprintf("            <TR BGCOLOR=\"%s\">\n", bgColor))
			content.WriteString(fmt.Sprintf("                <TD><B>%s</B></TD>\n", entry.Name))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", entry.Type))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", entry.Permissions))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", entry.Owner))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", entry.Group))
			content.WriteString(fmt.Sprintf("                <TD>%d bytes</TD>\n", entry.Size))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", entry.CreationTime))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", entry.ModificationTime))
			content.WriteString(fmt.Sprintf("                <TD>%d</TD>\n", entry.InodeNumber))
			content.WriteString("            </TR>\n")
		}

		content.WriteString("        </TABLE>\n")
		content.WriteString("    >];\n\n")

	}

	// Cierre del archivo DOT
	content.WriteString("}\n")

	return content.String()
}

// ============================================================================
// REPORTE JOURNALING
// ============================================================================

// GenerateJournalingReport genera el reporte del journaling mostrando todas las transacciones
func GenerateJournalingReport(userOutputPath string, partitionID string) error {
	fmt.Println("======INICIO REPORTE JOURNALING======")
	fmt.Printf("Partition ID: %s\n", partitionID)
	
	// Buscar la partición montada para obtener la ruta del disco
	mountedPartition, exists := DiskManagement.MountedPartitions[partitionID]
	if !exists {
		return fmt.Errorf("error: la partición con ID '%s' no está montada", partitionID)
	}

	// Abrir archivo del disco
	file, err := os.OpenFile(mountedPartition.Path, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("error al abrir el disco: %v", err)
	}
	defer file.Close()

	// Leer el superblock para obtener la estructura del sistema de archivos
	var mbr Structs.MBR
	file.Seek(0, 0)
	binary.Read(file, binary.LittleEndian, &mbr)

	// Obtener la partición correcta
	var partition Structs.Partition
	var partitionStart int64
	if mountedPartition.IsLogical {
		file.Seek(int64(mountedPartition.EBRPosition), 0)
		var ebr Structs.EBR
		binary.Read(file, binary.LittleEndian, &ebr)
		partition.Start = ebr.Part_start
		partition.Size = ebr.Part_size
		partitionStart = int64(ebr.Part_start)
	} else {
		partition = mbr.Partitions[mountedPartition.PartitionIndex]
		partitionStart = int64(partition.Start)
	}

	// Leer el superblock
	file.Seek(partitionStart, 0)
	var superblock Structs.Superblock
	binary.Read(file, binary.LittleEndian, &superblock)

	// Verificar que sea un sistema de archivos EXT3 (journaling)
	if superblock.S_filesystem_type != 3 {
		return fmt.Errorf("error: el sistema de archivos no es EXT3. Solo EXT3 soporta journaling")
	}

	// Usar la ruta exacta especificada por el usuario
	dotPath, imagePath := processUserPath(userOutputPath)

	// Crear directorio de salida si no existe
	if err := createOutputDirectory(filepath.Dir(dotPath)); err != nil {
		return err
	}

	// Generar contenido del reporte en formato DOT
	dotContent := generateJournalingDotContent(file, &superblock, partitionStart, partitionID)

	// Escribir archivo DOT
	if err := writeReportFile(dotPath, dotContent); err != nil {
		return err
	}

	// Generar imagen usando Graphviz
	if err := generateGraphvizImage(dotPath, imagePath); err != nil {
		fmt.Printf("Advertencia: No se pudo generar la imagen con Graphviz: %v\n", err)
		fmt.Printf("Archivo DOT generado en: %s\n", dotPath)
	} else {
		fmt.Printf("Reporte generado exitosamente:\n")
		fmt.Printf("  - Archivo DOT: %s\n", dotPath)
		fmt.Printf("  - Imagen: %s\n", imagePath)
	}

	fmt.Println("======FIN REPORTE JOURNALING======")
	return nil
}

// JournalingEntry representa una entrada en el journal
type JournalingEntry struct {
	Index     int
	Operation string
	Path      string
	Content   string
	Date      string
}

// generateJournalingDotContent genera el contenido del reporte JOURNALING en formato DOT
func generateJournalingDotContent(file *os.File, superblock *Structs.Superblock, partitionStart int64, partitionID string) string {
	var content strings.Builder

	// Encabezado del archivo DOT
	content.WriteString("digraph journaling_report {\n")
	content.WriteString("    // Configuración general\n")
	content.WriteString("    node [shape=plaintext, fontname=\"Arial\", fontsize=9];\n")
	content.WriteString("    rankdir=TB;\n")
	content.WriteString("    \n")

	// Título del reporte
	content.WriteString("    title [label=<\n")
	content.WriteString("        <TABLE BORDER=\"0\" CELLBORDER=\"0\" CELLSPACING=\"0\">\n")
	content.WriteString(fmt.Sprintf("            <TR><TD><FONT POINT-SIZE=\"16\"><B>REPORTE DE JOURNALING - Partición: %s</B></FONT></TD></TR>\n", partitionID))
	content.WriteString("        </TABLE>\n")
	content.WriteString("    >];\n\n")

	// Leer todas las entradas del journaling
	entries := readJournalingEntries(file, superblock, partitionStart)

	if len(entries) == 0 {
		// No hay entradas en el journal
		content.WriteString("    empty [label=<\n")
		content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
		content.WriteString("            <TR><TD BGCOLOR=\"#FFCDD2\"><B>JOURNAL VACÍO</B></TD></TR>\n")
		content.WriteString("            <TR><TD>No se encontraron transacciones registradas</TD></TR>\n")
		content.WriteString("        </TABLE>\n")
		content.WriteString("    >];\n")
		content.WriteString("    title -> empty [style=invis];\n")
	} else {
		// Tabla principal con las entradas del journal
		content.WriteString("    journal [label=<\n")
		content.WriteString("        <TABLE BORDER=\"1\" CELLBORDER=\"1\" CELLSPACING=\"0\" CELLPADDING=\"4\">\n")
		
		// Encabezado de la tabla
		content.WriteString("            <TR BGCOLOR=\"#2196F3\">\n")
		content.WriteString("                <TD><FONT COLOR=\"white\"><B>#</B></FONT></TD>\n")
		content.WriteString("                <TD><FONT COLOR=\"white\"><B>Operación</B></FONT></TD>\n")
		content.WriteString("                <TD><FONT COLOR=\"white\"><B>Ruta</B></FONT></TD>\n")
		content.WriteString("                <TD><FONT COLOR=\"white\"><B>Contenido</B></FONT></TD>\n")
		content.WriteString("                <TD><FONT COLOR=\"white\"><B>Fecha y Hora</B></FONT></TD>\n")
		content.WriteString("            </TR>\n")

		// Entries data
		for _, entry := range entries {
			// Alternar colores de fondo
			bgColor := "#E3F2FD"
			if entry.Index%2 == 0 {
				bgColor = "#FFFFFF"
			}

			// Truncar contenido si es muy largo
			displayContent := entry.Content
			if len(displayContent) > 50 {
				displayContent = displayContent[:50] + "..."
			}

			content.WriteString(fmt.Sprintf("            <TR BGCOLOR=\"%s\">\n", bgColor))
			content.WriteString(fmt.Sprintf("                <TD ALIGN=\"CENTER\">%d</TD>\n", entry.Index))
			content.WriteString(fmt.Sprintf("                <TD><B>%s</B></TD>\n", entry.Operation))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", escapeHTML(entry.Path)))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", escapeHTML(displayContent)))
			content.WriteString(fmt.Sprintf("                <TD>%s</TD>\n", entry.Date))
			content.WriteString("            </TR>\n")
		}

		content.WriteString("        </TABLE>\n")
		content.WriteString("    >];\n\n")

		// Información adicional
		content.WriteString("    info [label=<\n")
		content.WriteString("        <TABLE BORDER=\"0\" CELLBORDER=\"0\" CELLSPACING=\"0\">\n")
		content.WriteString(fmt.Sprintf("            <TR><TD><B>Total de transacciones:</B> %d</TD></TR>\n", len(entries)))
		content.WriteString("        </TABLE>\n")
		content.WriteString("    >];\n\n")

		content.WriteString("    title -> journal -> info [style=invis];\n")
	}

	// Cierre del archivo DOT
	content.WriteString("}\n")

	return content.String()
}

// readJournalingEntries lee todas las entradas del journaling
func readJournalingEntries(file *os.File, superblock *Structs.Superblock, partitionStart int64) []JournalingEntry {
	var entries []JournalingEntry

	// El journaling está ubicado después del superblock
	// Superblock ocupa 94 bytes (calculado con binary.Size)
	superblockSize := int64(binary.Size(Structs.Superblock{}))
	journalStart := partitionStart + superblockSize

	// Leer entradas del journal
	// Cada entrada de journaling tiene un count y una estructura Information
	const JOURNALING_CONSTANT = 50
	journalingSize := int64(binary.Size(Structs.Journaling{}))
	
	index := 1
	for i := 0; i < JOURNALING_CONSTANT; i++ {
		// Posición actual del journal
		currentPos := journalStart + int64(i)*journalingSize

		// Leer entrada del journaling
		file.Seek(currentPos, 0)
		var journal Structs.Journaling
		err := binary.Read(file, binary.LittleEndian, &journal)
		if err != nil {
			break
		}

		// Si count es 0, no hay más entradas válidas
		if journal.Count == 0 {
			continue
		}

		// Convertir bytes a strings
		operation := cleanString(journal.Content.Operation[:])
		path := cleanString(journal.Content.Path[:])
		contentStr := cleanString(journal.Content.Content[:])
		
		// Si la operación está vacía, skip
		if operation == "" {
			continue
		}
		
		// Convertir fecha de float32 a string legible
		dateStr := "N/A"
		if journal.Content.Date > 0 {
			// Convertir timestamp Unix a fecha
			dateStr = Utilities.ConvertUnixTimestamp(int64(journal.Content.Date))
		}

		// Agregar entrada
		entries = append(entries, JournalingEntry{
			Index:     index,
			Operation: operation,
			Path:      path,
			Content:   contentStr,
			Date:      dateStr,
		})

		index++
	}

	return entries
}

// escapeHTML escapa caracteres especiales para HTML
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
