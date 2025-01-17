package aeropuerto

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Estructuras
type Avion struct {
    ID           int
    Categoria    string
    NumPasajeros int
}

type TorreControl struct {
    mutex     sync.Mutex
    capacidad int
    cola      []*Avion
    avionesProcesando int
}

type Pista struct {
    ID        int
    ocupada   bool
    mutex     sync.Mutex
}

type Estado struct {
    valor     int
    mutex     sync.Mutex
}

// Variables globales
var (
    buf         bytes.Buffer
    logger      = log.New(&buf, "logger: ", log.Lshortfile)
    msg         string
    torre       TorreControl
    pistas      []Pista
    estado      Estado
    numPistas   = 3 
    maxEspera   = 5
    avionesChannel = make(chan *Avion, maxEspera)
    estadoInicial = true
)

// Constantes de tiempo (en segundos)
const (
    tiempoBaseAtencionTorre = 3
    tiempoBasePista        = 4
    tiempoBasePuerta       = 3
    variacionTiempo        = 2
)

func procesarAvion(avion *Avion) {
    torre.mutex.Lock()
    if torre.avionesProcesando >= maxEspera {
        torre.mutex.Unlock()
        fmt.Printf("Avión %d (Cat %s) en espera, torre congestionada\n", avion.ID, avion.Categoria)
        time.Sleep(time.Second * 5)
        select {
        case avionesChannel <- avion:
            // El avión fue reencolado exitosamente
        default:
            fmt.Printf("Avión %d (Cat %s) no pudo ser reencolado\n", avion.ID, avion.Categoria)
        }
        return
    }
    torre.avionesProcesando++
    torre.mutex.Unlock()

    fmt.Printf("Avión %d (Cat %s) solicitando permiso a torre\n", avion.ID, avion.Categoria)
    
    estado.mutex.Lock()
    estadoActual := estado.valor
    estado.mutex.Unlock()
    
    if !puedeAterrizar(avion, estadoActual) {
        fmt.Printf("Avión %d (Cat %s) no puede aterrizar en estado actual %d\n", avion.ID, avion.Categoria, estadoActual)
        torre.mutex.Lock()
        torre.avionesProcesando--
        torre.mutex.Unlock()
        time.Sleep(time.Second * 3)
        select {
        case avionesChannel <- avion:
            // El avión fue reencolado exitosamente
        default:
            fmt.Printf("Avión %d (Cat %s) abortó aterrizaje\n", avion.ID, avion.Categoria)
        }
        return
    }

    time.Sleep(time.Duration(tiempoBaseAtencionTorre) * time.Second)
    
    pistaAsignada := -1
    for i := range pistas {
        pistas[i].mutex.Lock()
        if !pistas[i].ocupada {
            pistas[i].ocupada = true
            pistaAsignada = i
            pistas[i].mutex.Unlock()
            break
        }
        pistas[i].mutex.Unlock()
    }

    if pistaAsignada >= 0 {
        fmt.Printf("Avión %d (Cat %s) aterrizando en pista %d\n", avion.ID, avion.Categoria, pistaAsignada)
        time.Sleep(time.Duration(tiempoBasePista) * time.Second)
        
        pistas[pistaAsignada].mutex.Lock()
        pistas[pistaAsignada].ocupada = false
        pistas[pistaAsignada].mutex.Unlock()
        
        fmt.Printf("Avión %d (Cat %s) en puerta de desembarque\n", avion.ID, avion.Categoria)
        time.Sleep(time.Duration(tiempoBasePuerta) * time.Second)
        
        fmt.Printf("Avión %d (Cat %s) ha completado operaciones\n", avion.ID, avion.Categoria)
    } else {
        fmt.Printf("Avión %d (Cat %s) esperando pista disponible\n", avion.ID, avion.Categoria)
        avionesChannel <- avion
    }

    torre.mutex.Lock()
    torre.avionesProcesando--
    torre.mutex.Unlock()
}

func puedeAterrizar(avion *Avion, estadoActual int) bool {
    switch estadoActual {
    case 0, 9:
        return false
    case 1:
        return avion.Categoria == "A"
    case 2:
        return avion.Categoria == "B"
    case 3:
        return avion.Categoria == "C"
    case 4:
        return avion.Categoria == "A" // Prioridad A
    case 5:
        return avion.Categoria == "B" // Prioridad B
    case 6:
        return avion.Categoria == "C" // Prioridad C
    case 7, 8:
        return true // Mantiene estado anterior, permite todos
    default:
        return true
    }
}

func determinarCategoria(numPasajeros int) string {
    if numPasajeros > 100 {
        return "A"
    } else if numPasajeros >= 50 {
        return "B"
    }
    return "C"
}

func gestionarAviones() {
    for avion := range avionesChannel {
        time.Sleep(time.Second * 2)
        go procesarAvion(avion)
    }
}

func Run() {
    conn, err := net.Dial("tcp", "localhost:8000")
    if err != nil {
        logger.Fatal(err)
    }
    defer conn.Close()

    rand.Seed(time.Now().UnixNano())
    
    pistas = make([]Pista, numPistas)
    for i := range pistas {
        pistas[i] = Pista{ID: i}
    }

    torre.capacidad = maxEspera
    estado.valor = 0

    // Esperar a recibir el primer estado antes de comenzar
    time.Sleep(time.Second * 2)

    go gestionarAviones()
    go func() {
        time.Sleep(time.Second * 3)
        generarAvionesTest(10, 10, 10)
    }()

    // Buffer para lectura del servidor
    reader := bufio.NewReader(conn)
    
    for {
        mensaje, err := reader.ReadString('\n')
        if err != nil {
            if err == io.EOF {
                break
            }
            fmt.Println("Error leyendo:", err)
            continue
        }
        
        mensaje = strings.TrimSpace(mensaje)
        procesarMensajeServidor(mensaje)
    }
}

func generarAvionesTest(numA, numB, numC int) {
    total := numA + numB + numC
    aviones := make([]*Avion, total)
    
    idx := 0
    for i := 0; i < numA; i++ {
        aviones[idx] = &Avion{
            ID: idx,
            NumPasajeros: 101 + rand.Intn(100),
        }
        aviones[idx].Categoria = determinarCategoria(aviones[idx].NumPasajeros)
        idx++
    }
    for i := 0; i < numB; i++ {
        aviones[idx] = &Avion{
            ID: idx,
            NumPasajeros: 50 + rand.Intn(51),
        }
        aviones[idx].Categoria = determinarCategoria(aviones[idx].NumPasajeros)
        idx++
    }
    for i := 0; i < numC; i++ {
        aviones[idx] = &Avion{
            ID: idx,
            NumPasajeros: 1 + rand.Intn(49),
        }
        aviones[idx].Categoria = determinarCategoria(aviones[idx].NumPasajeros)
        idx++
    }

    rand.Shuffle(len(aviones), func(i, j int) {
        aviones[i], aviones[j] = aviones[j], aviones[i]
    })

    for _, avion := range aviones {
        avionesChannel <- avion
        time.Sleep(time.Second * time.Duration(3+rand.Intn(3)))
    }
}

func procesarMensajeServidor(mensaje string) {
    // Si el mensaje está vacío o solo contiene caracteres de espacio, ignorarlo
    mensaje = strings.TrimSpace(mensaje)
    if mensaje == "" {
        return
    }

    // Intentar convertir el mensaje a número
    nuevoEstado, err := strconv.Atoi(mensaje)
    if err != nil {
        fmt.Printf("Error procesando mensaje del servidor: %s\n", mensaje)
        return
    }

    estado.mutex.Lock()
    estadoAnterior := estado.valor
    estado.valor = nuevoEstado
    estado.mutex.Unlock()

    // Solo mostrar cambios de estado significativos
    if estadoAnterior != nuevoEstado || estadoInicial {
        estadoInicial = false
        fmt.Printf("\n=== Cambio de estado del aeropuerto: %d -> %d ===\n", estadoAnterior, nuevoEstado)
        fmt.Printf("Significado: %s\n\n", interpretarEstado(nuevoEstado))
    }
}

// Nueva función para interpretar el significado del estado
func interpretarEstado(estado int) string {
    switch estado {
    case 0:
        return "Aeropuerto Inactivo"
    case 1:
        return "Solo Categoría A permitida"
    case 2:
        return "Solo Categoría B permitida"
    case 3:
        return "Solo Categoría C permitida"
    case 4:
        return "Prioridad para Categoría A"
    case 5:
        return "Prioridad para Categoría B"
    case 6:
        return "Prioridad para Categoría C"
    case 7, 8:
        return "Manteniendo estado anterior"
    case 9:
        return "Aeropuerto Cerrado Temporalmente"
    default:
        return "Estado no definido"
    }
}