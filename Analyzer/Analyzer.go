package Analyzer

import (
	"proyecto1/DiskManagement"
	"proyecto1/FileSystem"
	"proyecto1/Reportes"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"bytes"
	"io"
)

var re = regexp.MustCompile(`-(\w+)=("[^"]+"|\S+)`)

// ProcessCommandForAPI processes a single command for API usage and returns the output as string
func ProcessCommandForAPI(input string) string {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Buffer to capture output
	var buf bytes.Buffer
	done := make(chan bool)

	// Start a goroutine to copy the output
	go func() {
		io.Copy(&buf, r)
		done <- true
	}()

	// Check if input contains multiple commands (separated by newlines)
	lines := strings.Split(strings.TrimSpace(input), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		fmt.Printf(">>> Procesando: %s\n", line)
		processCommand(line)
		fmt.Println() // Add separator between commands
	}

	// Restore stdout
	w.Close()
	os.Stdout = old
	<-done

	return buf.String()
}



func processCommand(input string) {
	command, params := getCommandAndParams(input)

	if command == "exit" {
		fmt.Println("Comando exit recibido")
		return
	}

	fmt.Println("Ejecutando:", command, "con parámetros:", params)

	AnalyzeCommnad(command, params)
	
	fmt.Println()
}

func getCommandAndParams(input string) (string, string) {
	parts := strings.Fields(input)
	if len(parts) > 0 {
		command := strings.ToLower(parts[0])
		params := strings.Join(parts[1:], " ")
		return command, params
	}
	return "", input
}

func AnalyzeCommnad(command string, params string) {
	switch command {
	case "mkdisk":
		fn_mkdisk(params)
	case "rmdisk":
		fn_rmdisk(params)
	case "fdisk":
		fn_fdisk(params)
	case "mount":
		fn_mount(params)
	case "unmount":
		fn_unmount(params)
	case "mounted":
		fn_mounted(params)
	case "mkfs":
		fn_mkfs(params)
	case "rep":
		fn_rep(params)
	case "info":
		fn_info(params)
	case "ls":
		fn_ls(params)
	case "login":
		fn_login(params)
	case "logout":
		fn_logout(params)
	case "mkgrp":
		fn_mkgrp(params)
	case "rmgrp":
		fn_rmgrp(params)
	case "mkusr":
		fn_mkusr(params)
	case "rmusr":
		fn_rmusr(params)
	case "chgrp":
		fn_chgrp(params)
	case "mkfile":
		fn_mkfile(params)
	case "mkdir":
		fn_mkdir(params)
	case "cat":
		fn_cat(params)
	case "remove":
		fn_remove(params)
	case "edit":
		fn_edit(params)
	case "rename":
		fn_rename(params)
	case "copy":
		fn_copy(params)
	case "move":
		fn_move(params)
	case "find":
		fn_find(params)
	case "chown":
		fn_chown(params)
	case "chmod":
		fn_chmod(params)
	case "loss":
		fn_loss(params)
	case "recovery":
		fn_recovery(params)
	case "journaling":
		fn_journaling(params)
	case "exit":
		fmt.Println("Comando exit procesado - sesión terminada")
	default:
		fmt.Println("Error: Comando no reconocido.")
	}
}


func fn_mkdisk(params string) {
	// Definiendo banderas
	fs := flag.NewFlagSet("mkdisk", flag.ContinueOnError)
	fs.SetOutput(os.Stdout) // Para mostrar errores en stdout
	
	size := fs.Int("size", 0, "Size")
	fit := fs.String("fit", "ff", "Fit (opcional, default: ff)")
	unit := fs.String("unit", "m", "Unit (opcional, default: m)")
	path := fs.String("path", "", "Ruta donde crear el archivo (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *size <= 0 {
		fmt.Println("Error: El parámetro -size es requerido y debe ser mayor a 0")
		fmt.Println("Uso: mkdisk -size=<tamaño> -path=<ruta> [-unit=<k|m>] [-fit=<bf|ff|wf>]")
		return
	}

	if *path == "" {
		fmt.Println("Error: El parámetro -path es requerido")
		fmt.Println("Uso: mkdisk -size=<tamaño> -path=<ruta> [-unit=<k|m>] [-fit=<bf|ff|wf>]")
		return
	}

	// Llamar a la función
	DiskManagement.Mkdisk(*size, *fit, *unit, *path)
}

func fn_rmdisk(params string) {
	// Definiendo banderas
	fs := flag.NewFlagSet("rmdisk", flag.ContinueOnError)
	fs.SetOutput(os.Stdout) // Para mostrar errores en stdout
	
	path := fs.String("path", "", "Ruta del disco a eliminar (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *path == "" {
		fmt.Println("Error: El parámetro -path es requerido")
		fmt.Println("Uso: rmdisk -path=<ruta_del_disco>")
		fmt.Println("Ejemplo: rmdisk -path=\"/home/mis discos/Disco4.mia\"")
		return
	}

	// Llamar a la función
	DiskManagement.Rmdisk(*path)
}

func fn_fdisk(params string){
	//Definiendo parámetros
	fs := flag.NewFlagSet("fdisk", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	size := fs.Int("size", 0, "Tamaño de la partición")
	path := fs.String("path", "", "Ruta del disco")
	name := fs.String("name", "", "Nombre de la partición")
	type_ := fs.String("type", "p", "Tipo de la partición (p/e/l) (opcional, default: p)")
	fit := fs.String("fit", "wf", "Fit (opcional, default: ff)")
	unit := fs.String("unit", "k", "Unit (opcional, default: k)")
	add := fs.Int("add", 0, "Agregar o quitar espacio de la partición (opcional)")
	delete := fs.String("delete", "", "Eliminar partición (fast/full) (opcional)")

	// obtener valores
	managementFlags(fs, params)

	// Verificar si es una operación de eliminación
	if *delete != "" {
		if *path == "" || *name == "" {
			fmt.Println("Error: Los parámetros -path y -name son requeridos para eliminar una partición")
			fmt.Println("Uso: fdisk -delete=<fast|full> -path=<ruta> -name=<nombre>")
			return
		}
		DiskManagement.FdiskDelete(*path, *name, *delete)
		return
	}

	// Verificar si es una operación de agregar/quitar espacio
	if *add != 0 {
		if *path == "" || *name == "" {
			fmt.Println("Error: Los parámetros -path y -name son requeridos para modificar el espacio de una partición")
			fmt.Println("Uso: fdisk -add=<tamaño> -path=<ruta> -name=<nombre> [-unit=<b|k|m>]")
			return
		}
		DiskManagement.FdiskAdd(*path, *name, *add, *unit)
		return
	}

	// Validar parámetros requeridos para crear partición
	if *size <= 0 {
		fmt.Println("Error: El parámetro -size es requerido y debe ser mayor a 0")
		fmt.Println("Uso: fdisk -size=<tamaño> -path=<ruta> -name=<nombre> [-unit=<b|k|m>] [-type=<p|e|l>] [-fit=<b|f|w>]")
		return
	}
	if *path == "" {
		fmt.Println("Error: El parámetro -path es requerido")
		fmt.Println("Uso: fdisk -size=<tamaño> -path=<ruta> -name=<nombre> [-unit=<b|k|m>] [-type=<p|e|l>] [-fit=<b|f|w>]")
		return
	}
	if *name == "" {
		fmt.Println("Error: El parámetro -name es requerido")
		fmt.Println("Uso: fdisk -size=<tamaño> -path=<ruta> -name=<nombre> [-unit=<b|k|m>] [-type=<p|e|l>] [-fit=<b|f|w>]")
		return
	}

	//llamar a la función para crear partición
	DiskManagement.Fdisk(*size, *path, *name, *type_, *fit, *unit)
}

func fn_unmount(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("unmount", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	id := fs.String("id", "", "ID de la partición montada a desmontar (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *id == "" {
		fmt.Println("Error: El parámetro -id es requerido")
		fmt.Println("Uso: unmount -id=<id_particion>")
		fmt.Println("Ejemplo: unmount -id=851A")
		return
	}

	// Normalizar ID a mayúsculas para compatibilidad
	normalizedID := strings.ToUpper(*id)

	// Llamar la función
	DiskManagement.Unmount(normalizedID)
}

func fn_mounted(params string) {
	fmt.Println("======INICIO MOUNTED======")
	fmt.Println("Comando: mounted")
	fmt.Println("Descripción: Mostrar todas las particiones montadas en el sistema")
	fmt.Println()
	
	// Este comando no acepta parámetros
	if strings.TrimSpace(params) != "" {
		fmt.Println("Advertencia: El comando 'mounted' no acepta parámetros. Los parámetros serán ignorados.")
		fmt.Println()
	}
	
	DiskManagement.ShowDetailedMountedPartitions()
	fmt.Println("======FIN MOUNTED======")
}

func fn_mount(params string){
	//Definiendo parámetros
	fs := flag.NewFlagSet("mount", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	path := fs.String("path", "", "Ruta donde se encuentra el disco (obligatorio)")
	name := fs.String("name","","Nombre de la partición a montar (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *path == "" {
		fmt.Println("Error: El parámetro -path es requerido")
		fmt.Println("Uso: mount -path=<ruta_del_disco> -name=<nombre_particion>")
		fmt.Println("Ejemplo: mount -path=./test/A.mia -name=Particion1")
		return
	}
	if *name == "" {
		fmt.Println("Error: El parámetro -name es requerido")
		fmt.Println("Uso: mount -path=<ruta_del_disco> -name=<nombre_particion>")
		fmt.Println("Ejemplo: mount -path=./test/A.mia -name=Particion1")
		return
	}

	//llamar a la función
	DiskManagement.Mount(*path, *name)
}

func fn_mkfs(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("mkfs", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	id := fs.String("id", "", "ID de la partición montada (obligatorio)")
	type_ := fs.String("type", "full", "Tipo de formateo: full (opcional, default: full)")
	filesystem := fs.String("fs", "2fs", "Sistema de archivos: 2fs o 3fs (opcional, default: 2fs)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *id == "" {
		fmt.Println("Error: El parámetro -id es requerido")
		fmt.Println("Uso: mkfs -id=<ID_particion> [-type=full] [-fs=2fs|3fs]")
		fmt.Println("Ejemplo: mkfs -id=851A -type=full -fs=2fs")
		fmt.Println("Ejemplo: mkfs -id=851A -fs=3fs")
		return
	}

	// Validar que el tipo sea válido
	if *type_ != "full" {
		fmt.Printf("Error: Tipo '%s' no válido. Solo se acepta 'full'\n", *type_)
		fmt.Println("Uso: mkfs -id=<ID_particion> [-type=full] [-fs=2fs|3fs]")
		return
	}

	// Validar que el sistema de archivos sea válido
	if *filesystem != "2fs" && *filesystem != "3fs" {
		fmt.Printf("Error: Sistema de archivos '%s' no válido. Use '2fs' o '3fs'\n", *filesystem)
		fmt.Println("Uso: mkfs -id=<ID_particion> [-type=full] [-fs=2fs|3fs]")
		return
	}

	// Normalizar ID a mayúsculas para compatibilidad
	normalizedID := strings.ToUpper(*id)

	// Llamar la función
	FileSystem.Mkfs(normalizedID, *type_, *filesystem)
}

func fn_rep(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("rep", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	name := fs.String("name", "", "Nombre del reporte (obligatorio)")
	path := fs.String("path", "", "Ruta donde se generará el reporte (obligatorio)")
	id := fs.String("id", "", "ID de la partición (obligatorio)")
	path_file_ls := fs.String("path_file_ls", "", "Ruta del archivo o carpeta para reportes file y ls (opcional)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *name == "" {
		fmt.Println("Error: El parámetro -name es obligatorio")
		fmt.Println("Valores válidos: mbr, disk, inode, block, bm_inode, bm_block, tree, sb, file, ls")
		fmt.Println("Uso: rep -name=<tipo_reporte> -path=<ruta_salida> -id=<id_particion> [-path_file_ls=<ruta>]")
		return
	}

	if *path == "" {
		fmt.Println("Error: El parámetro -path es obligatorio")
		fmt.Println("Uso: rep -name=<tipo_reporte> -path=<ruta_salida> -id=<id_particion> [-path_file_ls=<ruta>]")
		return
	}

	if *id == "" {
		fmt.Println("Error: El parámetro -id es obligatorio")
		fmt.Println("Uso: rep -name=<tipo_reporte> -path=<ruta_salida> -id=<id_particion> [-path_file_ls=<ruta>]")
		return
	}

	// Validar que el tipo de reporte sea válido
	validReports := []string{"mbr", "disk", "inode", "block", "bm_inode", "bm_block", "tree", "sb", "file", "ls", "journaling"}
	reportType := strings.ToLower(*name)
	isValid := false
	for _, valid := range validReports {
		if reportType == valid {
			isValid = true
			break
		}
	}

	if !isValid {
		fmt.Printf("Error: Tipo de reporte '%s' no válido\n", *name)
		fmt.Println("Valores válidos: mbr, disk, inode, block, bm_inode, bm_block, tree, sb, file, ls, journaling")
		return
	}

	// Validar que path_file_ls se use solo con reportes file y ls
	if *path_file_ls != "" && reportType != "file" && reportType != "ls" {
		fmt.Printf("Advertencia: El parámetro -path_file_ls solo funciona con reportes 'file' y 'ls', se ignorará para el reporte '%s'\n", reportType)
		*path_file_ls = ""
	}

	// Validar que para reportes file y ls se proporcione path_file_ls si es necesario
	if (reportType == "file" || reportType == "ls") && *path_file_ls == "" {
		fmt.Printf("Advertencia: Para el reporte '%s' se recomienda usar el parámetro -path_file_ls\n", reportType)
	}

	// Normalizar ID a mayúsculas para compatibilidad
	normalizedID := strings.ToUpper(*id)

	fmt.Printf("Generando reporte '%s' con los siguientes parámetros:\n", reportType)
	fmt.Printf("  - Ruta de salida: %s\n", *path)
	fmt.Printf("  - ID partición: %s\n", normalizedID)
	if *path_file_ls != "" {
		fmt.Printf("  - Archivo/Carpeta: %s\n", *path_file_ls)
	}
	fmt.Println()

	// Generar el reporte según el tipo
	switch reportType {
	case "mbr":
		fmt.Printf("✓ Generando reporte MBR\n")
		if err := Reportes.GenerateMBRReport(*path, normalizedID); err != nil {
			fmt.Printf("Error generando reporte MBR: %v\n", err)
		}
	case "disk":
		fmt.Printf("✓ Generando reporte DISK\n")
		if err := Reportes.GenerateDiskReport(*path, normalizedID); err != nil {
			fmt.Printf("Error generando reporte DISK: %v\n", err)
		}
	case "inode":
		fmt.Printf("✓ Generando reporte INODE\n")
		if err := Reportes.GenerateInodeReport(*path, normalizedID); err != nil {
			fmt.Printf("Error generando reporte INODE: %v\n", err)
		}
	case "block":
		fmt.Printf("✓ Generando reporte BLOCK\n")
		if err := Reportes.GenerateBlockReport(*path, normalizedID); err != nil {
			fmt.Printf("Error generando reporte BLOCK: %v\n", err)
		}
	case "bm_inode":
		fmt.Printf("✓ Generando reporte BM_INODE\n")
		if err := Reportes.GenerateBitmapInodeReport(*path, normalizedID); err != nil {
			fmt.Printf("Error generando reporte BM_INODE: %v\n", err)
		}
	case "bm_block":
		fmt.Printf("✓ Generando reporte BM_BLOCK\n")
		if err := Reportes.GenerateBitmapBlockReport(*path, normalizedID); err != nil {
			fmt.Printf("Error generando reporte BM_BLOCK: %v\n", err)
		}
	case "tree":
		fmt.Printf("✓ Generando reporte TREE\n")
		if err := Reportes.GenerateTreeReport(*path, normalizedID); err != nil {
			fmt.Printf("Error generando reporte TREE: %v\n", err)
		}
	case "sb":
		fmt.Printf("✓ Generando reporte SB (SUPERBLOCK)\n")
		if err := Reportes.GenerateSuperblockReport(*path, normalizedID); err != nil {
			fmt.Printf("Error generando reporte SB: %v\n", err)
		}
	case "file":
		if *path_file_ls == "" {
			fmt.Printf("Error: Para el reporte FILE se requiere el parámetro -path_file_ls\n")
			fmt.Println("Uso: rep -name=file -path=<ruta_salida> -id=<id_particion> -path_file_ls=<ruta_archivo>")
			return
		}
		fmt.Printf("✓ Generando reporte FILE\n")
		if err := Reportes.GenerateFileReport(*path, normalizedID, *path_file_ls); err != nil {
			fmt.Printf("Error generando reporte FILE: %v\n", err)
		}
	case "ls":
		if *path_file_ls == "" {
			*path_file_ls = "/" // Directorio raíz por defecto
		}
		fmt.Printf("✓ Generando reporte LS\n")
		if err := Reportes.GenerateListReport(*path, normalizedID, *path_file_ls); err != nil {
			fmt.Printf("Error generando reporte LS: %v\n", err)
		}
	case "journaling":
		fmt.Printf("✓ Generando reporte JOURNALING\n")
		if err := Reportes.GenerateJournalingReport(*path, normalizedID); err != nil {
			fmt.Printf("Error generando reporte JOURNALING: %v\n", err)
		}
	default:
		// Para otros tipos de reporte, mostrar que están pendientes
		fmt.Printf("✓ Comando 'rep' reconocido correctamente para reporte tipo '%s'\n", reportType)
		fmt.Println("  [Implementación de generación de reportes pendiente]")
	}
}

func fn_info(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	id := fs.String("id", "", "Id de la partición montada")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *id == "" {
		fmt.Println("Error: El parámetro -id es requerido")
		return
	}

	// Llamar la función
	FileSystem.ShowFileSystemInfo(*id)
}

func fn_ls(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	id := fs.String("id", "", "Id de la partición montada")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *id == "" {
		fmt.Println("Error: El parámetro -id es requerido")
		return
	}

	// Llamar la función
	FileSystem.ListRootDirectory(*id)
}

func managementFlags(fs *flag.FlagSet, params string) {
	// NO usar fs.Parse(os.Args[1:]) porque estamos en modo interactivo
	
	// Encontrar las banderas en el input
	matches := re.FindAllStringSubmatch(params, -1)

	// Obtener los nombres de todas las banderas
	var flagNames []string
	var boolFlags []string
	fs.VisitAll(func(f *flag.Flag) {
		flagNames = append(flagNames, f.Name)
		// Detectar flags booleanos
		if f.DefValue == "false" {
			boolFlags = append(boolFlags, f.Name)
		}
	})

	// Buscar flags booleanos simples (como -p, -r) sin valor asignado
	boolRegex := regexp.MustCompile(`-(\w+)(?:\s|$)`)
	boolMatches := boolRegex.FindAllStringSubmatch(params, -1)
	
	for _, match := range boolMatches {
		flagName := match[1]
		
		// Verificar si es un flag booleano y no tiene valor asignado
		if contains(boolFlags, flagName) {
			// Verificar que no tenga = después
			valueRegex := regexp.MustCompile(`-` + flagName + `=`)
			if !valueRegex.MatchString(params) {
				err := fs.Set(flagName, "true")
				if err != nil {
					fmt.Printf("Error estableciendo valor booleano para %s: %v\n", flagName, err)
				}
			}
		}
	}

	// Procesar el comando ingresado con valores asignados
	for _, match := range matches {
		flagName := match[1]
		flagValue := match[2]

		// Remover comillas si existen
		flagValue = strings.Trim(flagValue, "\"")

		// Solo convertir a minúsculas parámetros específicos, NO paths
		if strings.ToLower(flagName) == "unit" || strings.ToLower(flagName) == "fit" || strings.ToLower(flagName) == "type" || strings.ToLower(flagName) == "fs" {
			flagValue = strings.ToLower(flagValue)
		}

		// Buscar el flag de manera case-insensitive
		actualFlagName := ""
		for _, fname := range flagNames {
			if strings.ToLower(fname) == strings.ToLower(flagName) {
				actualFlagName = fname
				break
			}
		}

		if actualFlagName != "" {
			err := fs.Set(actualFlagName, flagValue)
			if err != nil {
				fmt.Printf("Error estableciendo valor para %s: %v\n", actualFlagName, err)
			}
		} else {
			fmt.Println("Error: Bandera no encontrada:", flagName)
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func fn_login(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	user := fs.String("user", "", "Nombre del usuario (obligatorio)")
	pass := fs.String("pass", "", "Contraseña del usuario (obligatorio)")
	id := fs.String("id", "", "ID de la partición montada (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *user == "" {
		fmt.Println("Error: El parámetro -user es obligatorio")
		fmt.Println("Uso: login -user=<usuario> -pass=<contraseña> -id=<ID_particion>")
		fmt.Println("Ejemplo: login -user=root -pass=123 -id=851A")
		return
	}
	if *pass == "" {
		fmt.Println("Error: El parámetro -pass es obligatorio")
		fmt.Println("Uso: login -user=<usuario> -pass=<contraseña> -id=<ID_particion>")
		fmt.Println("Ejemplo: login -user=root -pass=123 -id=851A")
		return
	}
	if *id == "" {
		fmt.Println("Error: El parámetro -id es obligatorio")
		fmt.Println("Uso: login -user=<usuario> -pass=<contraseña> -id=<ID_particion>")
		fmt.Println("Ejemplo: login -user=root -pass=123 -id=851A")
		return
	}

	// Normalizar ID a mayúsculas para compatibilidad
	normalizedID := strings.ToUpper(*id)

	// Llamar la función
	FileSystem.Login(*user, *pass, normalizedID)
}

func fn_logout(params string) {
	// El comando logout no acepta parámetros
	if strings.TrimSpace(params) != "" {
		fmt.Println("Advertencia: El comando 'logout' no acepta parámetros. Los parámetros serán ignorados.")
	}
	
	// Llamar la función
	FileSystem.Logout()
}

func fn_mkgrp(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("mkgrp", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	name := fs.String("name", "", "Nombre del grupo (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *name == "" {
		fmt.Println("Error: El parámetro -name es obligatorio")
		fmt.Println("Uso: mkgrp -name=<nombre_grupo>")
		return
	}

	// Llamar la función
	FileSystem.Mkgrp(*name)
}

func fn_rmgrp(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("rmgrp", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	name := fs.String("name", "", "Nombre del grupo a eliminar (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *name == "" {
		fmt.Println("Error: El parámetro -name es obligatorio")
		fmt.Println("Uso: rmgrp -name=<nombre_grupo>")
		return
	}

	// Llamar la función
	FileSystem.Rmgrp(*name)
}

func fn_cat(params string) {
	// Si no hay parámetros, mostrar users.txt como antes
	if strings.TrimSpace(params) == "" {
		fmt.Println("Advertencia: Sin parámetros especificados. Mostrará el contenido de users.txt de la sesión actual.")
		FileSystem.CatUsersFile()
		return
	}
	
	// Definir flags para múltiples archivos
	fs := flag.NewFlagSet("cat", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	// Crear variables para hasta 10 archivos (extensible si es necesario)
	file1 := fs.String("file1", "", "Ruta del primer archivo")
	file2 := fs.String("file2", "", "Ruta del segundo archivo")
	file3 := fs.String("file3", "", "Ruta del tercer archivo")
	file4 := fs.String("file4", "", "Ruta del cuarto archivo")
	file5 := fs.String("file5", "", "Ruta del quinto archivo")
	file6 := fs.String("file6", "", "Ruta del sexto archivo")
	file7 := fs.String("file7", "", "Ruta del séptimo archivo")
	file8 := fs.String("file8", "", "Ruta del octavo archivo")
	file9 := fs.String("file9", "", "Ruta del noveno archivo")
	file10 := fs.String("file10", "", "Ruta del décimo archivo")

	// Obtener valores
	managementFlags(fs, params)

	// Recopilar todas las rutas de archivos especificadas
	var filePaths []string
	fileParams := []*string{file1, file2, file3, file4, file5, file6, file7, file8, file9, file10}
	
	for _, fileParam := range fileParams {
		if *fileParam != "" {
			filePaths = append(filePaths, *fileParam)
		}
	}

	// Verificar que se especificó al menos un archivo
	if len(filePaths) == 0 {
		fmt.Println("Error: Debe especificar al menos un archivo")
		fmt.Println("Uso: cat -file1=/ruta/archivo1 [-file2=/ruta/archivo2] ...")
		fmt.Println("Ejemplo: cat -file1=/home/user/docs/a.txt")
		fmt.Println("Ejemplo: cat -file1=/home/a.txt -file2=/home/b.txt -file3=/home/c.txt")
		return
	}

	// Llamar la función de cat con múltiples archivos
	FileSystem.Cat(filePaths)
}

func fn_mkusr(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("mkusr", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	user := fs.String("user", "", "Nombre del usuario (obligatorio, máximo 10 caracteres)")
	pass := fs.String("pass", "", "Contraseña del usuario (obligatorio, máximo 10 caracteres)")
	grp := fs.String("grp", "", "Grupo del usuario (obligatorio, máximo 10 caracteres)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *user == "" {
		fmt.Println("Error: El parámetro -user es obligatorio")
		fmt.Println("Uso: mkusr -user=<nombre_usuario> -pass=<contraseña> -grp=<nombre_grupo>")
		return
	}
	if *pass == "" {
		fmt.Println("Error: El parámetro -pass es obligatorio")
		fmt.Println("Uso: mkusr -user=<nombre_usuario> -pass=<contraseña> -grp=<nombre_grupo>")
		return
	}
	if *grp == "" {
		fmt.Println("Error: El parámetro -grp es obligatorio")
		fmt.Println("Uso: mkusr -user=<nombre_usuario> -pass=<contraseña> -grp=<nombre_grupo>")
		return
	}

	// Llamar la función
	FileSystem.Mkusr(*user, *pass, *grp)
}

func fn_rmusr(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("rmusr", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	user := fs.String("user", "", "Nombre del usuario a eliminar (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *user == "" {
		fmt.Println("Error: El parámetro -user es obligatorio")
		fmt.Println("Uso: rmusr -user=<nombre_usuario>")
		return
	}

	// Llamar la función
	FileSystem.Rmusr(*user)
}

func fn_chgrp(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("chgrp", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	user := fs.String("user", "", "Nombre del usuario al que cambiar el grupo (obligatorio)")
	grp := fs.String("grp", "", "Nombre del nuevo grupo (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *user == "" {
		fmt.Println("Error: El parámetro -user es obligatorio")
		fmt.Println("Uso: chgrp -user=<nombre_usuario> -grp=<nombre_grupo>")
		fmt.Println("Ejemplo: chgrp -user=juan -grp=administradores")
		return
	}
	if *grp == "" {
		fmt.Println("Error: El parámetro -grp es obligatorio")
		fmt.Println("Uso: chgrp -user=<nombre_usuario> -grp=<nombre_grupo>")
		fmt.Println("Ejemplo: chgrp -user=juan -grp=administradores")
		return
	}

	// Llamar la función
	FileSystem.Chgrp(*user, *grp)
}

func fn_mkfile(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("mkfile", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	path := fs.String("path", "", "Ruta del archivo a crear (obligatorio)")
	r := fs.Bool("r", false, "Crear directorios padre si no existen")
	size := fs.Int("size", 0, "Tamaño en bytes del archivo (opcional)")
	cont := fs.String("cont", "", "Archivo con contenido a copiar (opcional)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *path == "" {
		fmt.Println("Error: El parámetro -path es obligatorio")
		fmt.Println("Uso: mkfile -path=<ruta_archivo> [-r] [-size=<tamaño>] [-cont=<archivo_contenido>]")
		fmt.Println("Ejemplo: mkfile -path=/test.txt -size=10")
		fmt.Println("Ejemplo: mkfile -path=/archivo.txt -cont=/home/user/documento.txt")
		fmt.Println("Ejemplo: mkfile -path=/home/user/docs/archivo.txt -r -size=100")
		return
	}

	// Validar que el tamaño no sea negativo
	if *size < 0 {
		fmt.Println("Error: El tamaño del archivo no puede ser negativo")
		fmt.Println("Uso: mkfile -path=<ruta_archivo> [-r] [-size=<tamaño>] [-cont=<archivo_contenido>]")
		return
	}

	// Llamar la función
	FileSystem.Mkfile(*path, *r, *size, *cont)
}

func fn_mkdir(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("mkdir", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	path := fs.String("path", "", "Ruta del directorio a crear (obligatorio)")
	p := fs.Bool("p", false, "Crear directorios padre si no existen")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *path == "" {
		fmt.Println("Error: El parámetro -path es obligatorio")
		fmt.Println("Uso: mkdir -path=<ruta_directorio> [-p]")
		fmt.Println("Ejemplo: mkdir -path=/docs")
		fmt.Println("Ejemplo: mkdir -path=/home/user/documents -p")
		return
	}

	// Llamar la función
	FileSystem.Mkdir(*path, *p)
}

func fn_remove(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	path := fs.String("path", "", "Ruta del archivo o directorio a eliminar (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *path == "" {
		fmt.Println("Error: El parámetro -path es obligatorio")
		fmt.Println("Uso: remove -path=<ruta>")
		fmt.Println("Ejemplo: remove -path=/home/user/docs/a.txt")
		fmt.Println("Ejemplo: remove -path=\"/carpeta con espacios/archivo.txt\"")
		return
	}

	// Llamar la función
	FileSystem.Remove(*path)
}

func fn_edit(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	path := fs.String("path", "", "Ruta del archivo a editar (obligatorio)")
	contenido := fs.String("contenido", "", "Ruta del archivo local con el nuevo contenido (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *path == "" {
		fmt.Println("Error: El parámetro -path es obligatorio")
		fmt.Println("Uso: edit -path=<ruta_archivo> -contenido=<archivo_local>")
		fmt.Println("Ejemplo: edit -path=/home/user/docs/a.txt -contenido=/root/user/files/a.txt")
		return
	}

	if *contenido == "" {
		fmt.Println("Error: El parámetro -contenido es obligatorio")
		fmt.Println("Uso: edit -path=<ruta_archivo> -contenido=<archivo_local>")
		fmt.Println("Ejemplo: edit -path=/home/user/docs/a.txt -contenido=/root/user/files/a.txt")
		return
	}

	// Llamar la función
	FileSystem.Edit(*path, *contenido)
}

func fn_rename(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("rename", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	path := fs.String("path", "", "Ruta del archivo o directorio a renombrar (obligatorio)")
	name := fs.String("name", "", "Nuevo nombre para el archivo o directorio (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *path == "" {
		fmt.Println("Error: El parámetro -path es obligatorio")
		fmt.Println("Uso: rename -path=<ruta> -name=<nuevo_nombre>")
		fmt.Println("Ejemplo: rename -path=/home/user/docs/a.txt -name=b1.txt")
		return
	}

	if *name == "" {
		fmt.Println("Error: El parámetro -name es obligatorio")
		fmt.Println("Uso: rename -path=<ruta> -name=<nuevo_nombre>")
		fmt.Println("Ejemplo: rename -path=/home/user/docs/a.txt -name=b1.txt")
		return
	}

	// Llamar la función
	FileSystem.Rename(*path, *name)
}

func fn_copy(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("copy", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	path := fs.String("path", "", "Ruta del archivo o directorio a copiar (obligatorio)")
	destino := fs.String("destino", "", "Ruta de destino donde se copiará (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *path == "" {
		fmt.Println("Error: El parámetro -path es obligatorio")
		fmt.Println("Uso: copy -path=<ruta_origen> -destino=<ruta_destino>")
		fmt.Println("Ejemplo: copy -path=/home/user/documents -destino=/home/images")
		return
	}

	if *destino == "" {
		fmt.Println("Error: El parámetro -destino es obligatorio")
		fmt.Println("Uso: copy -path=<ruta_origen> -destino=<ruta_destino>")
		fmt.Println("Ejemplo: copy -path=/home/user/documents -destino=/home/images")
		return
	}

	// Llamar la función
	FileSystem.Copy(*path, *destino)
}

func fn_move(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("move", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	path := fs.String("path", "", "Ruta del archivo o directorio a mover (obligatorio)")
	destino := fs.String("destino", "", "Ruta de destino donde se moverá (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *path == "" {
		fmt.Println("Error: El parámetro -path es obligatorio")
		fmt.Println("Uso: move -path=<ruta_origen> -destino=<ruta_destino>")
		fmt.Println("Ejemplo: move -path=/home/user/documents -destino=/home/backup")
		return
	}

	if *destino == "" {
		fmt.Println("Error: El parámetro -destino es obligatorio")
		fmt.Println("Uso: move -path=<ruta_origen> -destino=<ruta_destino>")
		fmt.Println("Ejemplo: move -path=/home/user/documents -destino=/home/backup")
		return
	}

	// Llamar la función
	FileSystem.Move(*path, *destino)
}

func fn_find(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("find", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	path := fs.String("path", "", "Ruta donde iniciar la búsqueda (obligatorio)")
	name := fs.String("name", "", "Patrón de búsqueda con soporte para ? y * (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *path == "" {
		fmt.Println("Error: El parámetro -path es obligatorio")
		fmt.Println("Uso: find -path=<ruta_búsqueda> -name=<patrón>")
		fmt.Println("Ejemplo: find -path=/home -name=*.txt")
		fmt.Println("Comodines: ? (un carácter), * (uno o más caracteres)")
		return
	}

	if *name == "" {
		fmt.Println("Error: El parámetro -name es obligatorio")
		fmt.Println("Uso: find -path=<ruta_búsqueda> -name=<patrón>")
		fmt.Println("Ejemplo: find -path=/home -name=?.txt")
		fmt.Println("Comodines: ? (un carácter), * (uno o más caracteres)")
		return
	}

	// Llamar la función
	FileSystem.Find(*path, *name)
}

func fn_chown(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("chown", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	path := fs.String("path", "", "Ruta del archivo o directorio (obligatorio)")
	r := fs.Bool("r", false, "Cambiar propietario recursivamente (opcional)")
	usuario := fs.String("usuario", "", "Nombre del nuevo propietario (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *path == "" {
		fmt.Println("Error: El parámetro -path es obligatorio")
		fmt.Println("Uso: chown -path=<ruta> -usuario=<usuario> [-r]")
		fmt.Println("Ejemplo: chown -path=/home -usuario=user2 -r")
		return
	}

	if *usuario == "" {
		fmt.Println("Error: El parámetro -usuario es obligatorio")
		fmt.Println("Uso: chown -path=<ruta> -usuario=<usuario> [-r]")
		fmt.Println("Ejemplo: chown -path=/home/file.txt -usuario=user1")
		return
	}

	// Llamar la función
	FileSystem.Chown(*path, *r, *usuario)
}

func fn_chmod(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("chmod", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	path := fs.String("path", "", "Ruta del archivo o directorio (obligatorio)")
	ugo := fs.String("ugo", "", "Permisos en formato [0-7][0-7][0-7] (obligatorio)")
	r := fs.Bool("r", false, "Cambiar permisos recursivamente (opcional)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *path == "" {
		fmt.Println("Error: El parámetro -path es obligatorio")
		fmt.Println("Uso: chmod -path=<ruta> -ugo=<permisos> [-r]")
		fmt.Println("Ejemplo: chmod -path=/home -ugo=764 -r")
		return
	}

	if *ugo == "" {
		fmt.Println("Error: El parámetro -ugo es obligatorio")
		fmt.Println("Uso: chmod -path=<ruta> -ugo=<permisos> [-r]")
		fmt.Println("Formato: -ugo=[0-7][0-7][0-7] (Usuario, Grupo, Otros)")
		fmt.Println("Ejemplo: chmod -path=/home/file.txt -ugo=777")
		return
	}

	// Llamar la función
	FileSystem.Chmod(*path, *ugo, *r)
}

func fn_loss(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("loss", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	id := fs.String("id", "", "ID de la partición a formatear (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *id == "" {
		fmt.Println("Error: El parámetro -id es obligatorio")
		fmt.Println("Uso: loss -id=<id>")
		fmt.Println("Ejemplo: loss -id=851A")
		fmt.Println("\nEste comando simula un fallo en el disco formateando:")
		fmt.Println("  - Bitmap de Inodos")
		fmt.Println("  - Bitmap de Bloques")
		fmt.Println("  - Área de Inodos")
		fmt.Println("  - Área de Bloques")
		return
	}

	// Normalizar ID a mayúsculas para compatibilidad
	normalizedID := strings.ToUpper(*id)

	// Llamar la función
	FileSystem.Loss(normalizedID)
}

func fn_recovery(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("recovery", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	id := fs.String("id", "", "ID de la partición a recuperar (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *id == "" {
		fmt.Println("Error: El parámetro -id es obligatorio")
		fmt.Println("Uso: recovery -id=<id>")
		fmt.Println("Ejemplo: recovery -id=851A")
		fmt.Println("\nEste comando recupera el sistema de archivos EXT3 usando el journaling")
		fmt.Println("Restaura el sistema a un estado consistente antes del último formateo")
		return
	}

	// Normalizar ID a mayúsculas para compatibilidad
	normalizedID := strings.ToUpper(*id)

	// Llamar la función
	FileSystem.Recovery(normalizedID)
}

func fn_journaling(params string) {
	// Definir banderas
	fs := flag.NewFlagSet("journaling", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	
	id := fs.String("id", "", "ID de la partición (obligatorio)")

	// obtener valores
	managementFlags(fs, params)

	// Validar parámetros requeridos
	if *id == "" {
		fmt.Println("Error: El parámetro -id es obligatorio")
		fmt.Println("Uso: journaling -id=<id>")
		fmt.Println("Ejemplo: journaling -id=851A")
		fmt.Println("\nEste comando genera un reporte del journaling mostrando todas las transacciones")
		return
	}

	// Normalizar ID a mayúsculas para compatibilidad
	normalizedID := strings.ToUpper(*id)

	// Generar el reporte en la carpeta de reportes por defecto
	reportPath := "/home/jose/Documentos/proyecto2/reportes/journaling_report"
	fmt.Printf("✓ Generando reporte JOURNALING en: %s\n", reportPath)
	err := Reportes.GenerateJournalingReport(reportPath, normalizedID)
	if err != nil {
		fmt.Printf("Error al generar reporte de journaling: %v\n", err)
		return
	}
}
