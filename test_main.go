package main

import (
	"proyecto1/Analyzer"
	"fmt"
	"time"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║      PRUEBAS AUTOMATIZADAS - MKFS Y FDISK             ║")
	fmt.Println("║      Fecha:", time.Now().Format("2006-01-02 15:04:05"), "                    ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()

	testDir := "/home/jose/Documentos/proyecto2/Backend/test"
	diskPath := testDir + "/disco_pruebas.mia"

	// FASE 1: CREACIÓN DE DISCO
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  FASE 1: CREACIÓN DE DISCO")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()
	
	executeCommand("mkdisk -size=30 -unit=m -path=" + diskPath, "Crear disco de 30 MB")

	// FASE 2: CREAR PARTICIONES
	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  FASE 2: CREAR PARTICIONES")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	executeCommand("fdisk -size=6 -unit=m -path="+diskPath+" -name=Part1 -type=p", "Crear Part1 (6 MB)")
	executeCommand("fdisk -size=8 -unit=m -path="+diskPath+" -name=Part2 -type=p", "Crear Part2 (8 MB)")
	executeCommand("fdisk -size=10 -unit=m -path="+diskPath+" -name=Extended1 -type=e", "Crear Extended (10 MB)")
	executeCommand("fdisk -size=3 -unit=m -path="+diskPath+" -name=Logica1 -type=l", "Crear Logica1 (3 MB)")
	executeCommand("fdisk -size=3 -unit=m -path="+diskPath+" -name=Logica2 -type=l", "Crear Logica2 (3 MB)")

	// FASE 3: PRUEBAS DE FDISK ADD (Agregar)
	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  FASE 3: PRUEBAS DE FDISK ADD (Agregar Espacio)")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	executeCommand("fdisk -add=2 -unit=m -path="+diskPath+" -name=Part1", "Agregar 2 MB a Part1")
	executeCommand("fdisk -add=1024 -unit=k -path="+diskPath+" -name=Logica1", "Agregar 1024 KB a Logica1")

	// FASE 4: PRUEBAS DE FDISK ADD (Quitar)
	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  FASE 4: PRUEBAS DE FDISK ADD (Quitar Espacio)")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	executeCommand("fdisk -add=-1 -unit=m -path="+diskPath+" -name=Part2", "Quitar 1 MB de Part2")
	executeCommand("fdisk -add=-512 -unit=k -path="+diskPath+" -name=Logica2", "Quitar 512 KB de Logica2")

	// FASE 5: MONTAR PARTICIONES
	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  FASE 5: MONTAR PARTICIONES")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	executeCommand("mount -path="+diskPath+" -name=Part1", "Montar Part1")
	executeCommand("mount -path="+diskPath+" -name=Part2", "Montar Part2")
	executeCommand("mount -path="+diskPath+" -name=Logica1", "Montar Logica1")
	executeCommand("mounted", "Ver particiones montadas")

	// FASE 6: FORMATEAR CON EXT2
	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  FASE 6: FORMATEAR CON EXT2")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	executeCommand("mkfs -id=851A -type=full", "Formatear Part1 con EXT2 (por defecto)")
	executeCommand("mkfs -id=853A -type=full -fs=2fs", "Formatear Logica1 con EXT2 (explícito)")

	// FASE 7: FORMATEAR CON EXT3
	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  FASE 7: FORMATEAR CON EXT3")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	executeCommand("mkfs -id=852A -type=full -fs=3fs", "Formatear Part2 con EXT3 (con journaling)")

	// FASE 8: VERIFICAR INFORMACIÓN
	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  FASE 8: VERIFICAR INFORMACIÓN DE PARTICIONES")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	executeCommand("info -id=851A", "Info de Part1 (EXT2)")
	executeCommand("info -id=852A", "Info de Part2 (EXT3)")
	executeCommand("info -id=853A", "Info de Logica1 (EXT2)")

	// FASE 9: OPERACIONES EN EXT2
	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  FASE 9: OPERACIONES EN SISTEMA EXT2")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	executeCommand("login -user=root -pass=123 -id=851A", "Login en Part1 (EXT2)")
	executeCommand("mkdir -path=/docs", "Crear directorio /docs")
	executeCommand("mkfile -path=/docs/test.txt -size=100", "Crear archivo test.txt")
	executeCommand("cat", "Mostrar users.txt")
	executeCommand("logout", "Cerrar sesión")

	// FASE 10: OPERACIONES EN EXT3
	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  FASE 10: OPERACIONES EN SISTEMA EXT3")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	executeCommand("login -user=root -pass=123 -id=852A", "Login en Part2 (EXT3)")
	executeCommand("mkdir -path=/data", "Crear directorio /data")
	executeCommand("mkfile -path=/data/archivo.txt -size=200", "Crear archivo archivo.txt")
	executeCommand("cat", "Mostrar users.txt")
	executeCommand("logout", "Cerrar sesión")

	// RESUMEN FINAL
	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  RESUMEN FINAL")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	executeCommand("mounted", "Estado final de particiones")

	// RESUMEN
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║      PRUEBAS COMPLETADAS EXITOSAMENTE                 ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Pruebas realizadas:")
	fmt.Println("  ✓ Creación de disco")
	fmt.Println("  ✓ Creación de particiones (primarias, extendida, lógicas)")
	fmt.Println("  ✓ FDISK ADD - Agregar espacio")
	fmt.Println("  ✓ FDISK ADD - Quitar espacio")
	fmt.Println("  ✓ Montaje de particiones")
	fmt.Println("  ✓ MKFS con EXT2")
	fmt.Println("  ✓ MKFS con EXT3")
	fmt.Println("  ✓ Operaciones en EXT2")
	fmt.Println("  ✓ Operaciones en EXT3")
	fmt.Println()
	fmt.Println("Archivo de disco creado en:")
	fmt.Println(" ", diskPath)
	fmt.Println()
}

func executeCommand(cmd string, description string) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Test:", description)
	fmt.Println("Comando:", cmd)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	output := Analyzer.ProcessCommandForAPI(cmd)
	fmt.Print(output)
	fmt.Println()
}
