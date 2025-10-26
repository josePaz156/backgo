package Utilities

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

//función para crear el archivo binario
func CreateFile(name string) error {
	dir := filepath.Dir(name)
	
	// Crear directorios si no existen
	if dir != "." && dir != "" {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				fmt.Printf("Error creando directorio %s: %v\n", dir, err)
				return err
			}
		}
	}
	
	// Crear archivo si no existe
	if _, err := os.Stat(name); os.IsNotExist(err) {
		file, err := os.Create(name)
		if err != nil {
			fmt.Printf("Error creando archivo %s: %v\n", name, err)
			return err
		}
		defer file.Close()
	} else {
		fmt.Printf("Advertencia: El archivo %s ya existe, será sobrescrito\n", name)
	}
	
	return nil
}

//función para abrir el archivo binario en modo lectura/escritura
func OpenFile(name string) (*os.File, error) {
	file, err := os.OpenFile(name, os.O_RDWR, 0644)
	if err != nil {
		fmt.Println("Error abriendo archivo:", err)
		return nil, err
	}
	return file, nil
}

//función para escribir el objeto en el archivo binario
func WriteObject(file *os.File, data interface{}, position int64) error {
	file.Seek(position, 0)
	err := binary.Write(file, binary.LittleEndian, data)
	if err != nil {
		fmt.Println("Error escribiendo el archivo:", err)
		return err
	}
	return nil
}

//Función para leer los objetos desde el archivo binario
func ReadObject(file *os.File, data interface{}, position int64) error {
	file.Seek(position, 0)
	err := binary.Read(file, binary.LittleEndian, data)
	if err != nil {
		fmt.Println("Error leyendo el objeto del archivo binario", err)
		return err
	}
	return nil
}
// ConvertUnixTimestamp convierte un timestamp Unix a formato legible
func ConvertUnixTimestamp(timestamp int64) string {
if timestamp == 0 {
return "N/A"
}
t := time.Unix(timestamp, 0)
return t.Format("2006-01-02 15:04:05")
}
