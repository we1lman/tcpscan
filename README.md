# tcpscan

[![ci](https://github.com/we1lman/tcpscan/actions/workflows/ci.yml/badge.svg)](https://github.com/we1lman/tcpscan/actions/workflows/ci.yml)

**Русский** · [English](#english)

Go-пакет для проверки доступности TCP-портов на одном или нескольких хостах.

Выполняет обычное TCP Connect-сканирование средствами стандартной библиотеки: пакет просит операционную систему установить соединение и сразу его закрывает. Raw-сокеты и права администратора не нужны.

Пакет пригоден для встраивания в другие приложения и не зависит ни от CLI, ни от HTTP-фреймворка, ни от базы данных, ни от брокера сообщений.

## Требования

Go 1.25.5 или новее.

## Установка

```bash
go get github.com/we1lman/tcpscan
```

## Быстрый старт

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/we1lman/tcpscan"
)

func main() {
	scanner, err := tcpscan.New(
		tcpscan.WithConcurrency(100),
		tcpscan.WithConnectTimeout(500*time.Millisecond),
	)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results, err := scanner.Scan(
		ctx,
		[]string{"127.0.0.1", "example.com"},
		tcpscan.Range(1, 1024),
	)
	if err != nil {
		panic(err)
	}

	for r := range results {
		if r.State != tcpscan.StateOpen {
			continue
		}
		fmt.Printf("%s (%s):%d открыт за %s\n", r.Host, r.IP, r.Port, r.Duration)
	}
}
```

## Устройство публичного API

Публичный API состоит из четырёх частей: сканер, опции, наборы портов и результаты.

### Сканер

```go
func New(opts ...Option) (*Scanner, error)
func (s *Scanner) Scan(ctx context.Context, hosts []string, ports PortSet) (<-chan Result, error)
```

`New` создаёт сканер, применяя опции поверх значений по умолчанию. Первая же неверная опция прерывает создание, и возвращается `nil`. Опция со значением `nil` пропускается.

`Scanner` после создания неизменяем. Его можно использовать повторно и вызывать `Scan` из нескольких горутин одновременно.

`Scan` проверяет каждый порт набора на каждой цели и отдаёт результаты потоком.

Ошибки разделены на два вида:

| Вид | Как приходит | Примеры |
|---|---|---|
| Невалидный вход | Возвращаемым значением, синхронно | пустой список целей, порт 0, обратный диапазон |
| Сбой по ходу работы | Результатом в канале | порт закрыт, таймаут, имя не разрешилось |

Вход проверяется до запуска работы: если `Scan` вернул ошибку, канал не создаётся и ни одного соединения не открывается.

### Опции

```go
func WithConcurrency(n int) Option
func WithConnectTimeout(d time.Duration) Option
```

| Опция | Допустимые значения | По умолчанию |
|---|---|---|
| `WithConcurrency` | 1–65535 | 100 |
| `WithConnectTimeout` | больше нуля | 2s |

`WithConcurrency` задаёт максимальное число одновременных попыток подключения. `WithConnectTimeout` ограничивает время **одной** проверки; общий срок на весь скан задаётся контекстом.

Тип `Option` экспортирован, но его реализация закрыта: определять свои опции вне пакета нельзя, поэтому любая опция, попавшая в `New`, прошла валидацию.

### Наборы портов

```go
func Ports(ports ...int) PortSet
func Range(from, to int) PortSet
func Union(sets ...PortSet) PortSet
func ParsePorts(spec string) PortSet

func (s PortSet) Err() error
func (s PortSet) Len() int
```

`PortSet` — отсортированное множество портов без дубликатов, неизменяемое и безопасное для одновременного использования несколькими сканами.

```go
tcpscan.Ports(22, 80, 443)
tcpscan.Range(1, 1024)
tcpscan.Union(tcpscan.Range(20, 25), tcpscan.Ports(80, 443))
tcpscan.ParsePorts("22,80,443,8000-8100")
```

**Валидация отложенная.** Конструктор никогда не завершается ошибкой на месте — он сохраняет проблему внутри набора. Это сделано ради того, чтобы набор можно было собирать прямо в аргументах `Scan`, как в примере выше. Ошибку возвращает `Err()` либо сам `Scan`.

Формат для `ParsePorts` — список элементов через запятую, где элемент это порт или диапазон: `80`, `22,80,443`, `1-1024`, `20-25,80,8000-8100`. Пробелы вокруг элементов и вокруг дефиса игнорируются.

### Результаты

```go
type Result struct {
	Host     string
	IP       net.IP
	Port     uint16
	State    State
	Duration time.Duration
	Err      error
}
```

`Host` — цель в том виде, в каком её передал вызывающий, с обрезанными пробелами по краям. `IP` — адрес, на который реально шло подключение; отличается от `Host`, когда целью было DNS-имя.

Если цель не удалось разрешить, на неё создаётся один результат: заполнены `Host` и `Err`, поля `IP`, `Port` и `Duration` остаются нулевыми.

### Состояния

```go
const (
	StateUnknown State = iota
	StateOpen
	StateClosed
	StateTimeout
	StateUnreachable
	StateCanceled
	StateError
)
```

| Состояние | Что произошло |
|---|---|
| `open` | TCP-соединение установлено |
| `closed` | Узел ответил отказом — на порту никто не слушает |
| `timeout` | Узел молчал дольше таймаута подключения |
| `unreachable` | До узла или до сети нет пути |
| `canceled` | Контекст отменён до завершения проверки |
| `error` | Прочие ошибки, в том числе неудачный резолв имени |
| `unknown` | Нулевое значение, результат не заполнялся |

Состояние определяется **только по типам ошибок** — через `errors.Is` и `errors.As` по значениям `context.Canceled`, `context.DeadlineExceeded`, интерфейсу `net.Error`, типу `*net.DNSError` и системным кодам `syscall.Errno`. Сравнения текста ошибки в коде нет; это зафиксировано тестом `TestClassifyIgnoresErrorText`.

Ключевое различие: `closed` — это **ответ** узла, `timeout` — **молчание**. Из молчания однозначных выводов не следует.

### Ошибки

```go
var (
	ErrNoPorts       = errors.New("tcpscan: no ports specified")
	ErrInvalidPort   = errors.New("tcpscan: invalid port")
	ErrInvalidRange  = errors.New("tcpscan: invalid port range")
	ErrNoTargets     = errors.New("tcpscan: no targets specified")
	ErrInvalidTarget = errors.New("tcpscan: invalid target")
	ErrInvalidOption = errors.New("tcpscan: invalid option")
)

type TargetError struct {
	Input string
	Err   error
}
```

Все ошибки сравниваются через `errors.Is`. `TargetError` дополнительно сообщает, **какая именно** цель оказалась проблемной, и реализует `Unwrap`, поэтому обе формы работают одновременно:

```go
if errors.Is(err, tcpscan.ErrInvalidTarget) {
	// что случилось
}

var targetErr *tcpscan.TargetError
if errors.As(err, &targetErr) {
	// с какой целью это случилось
	log.Printf("проблемная цель: %s", targetErr.Input)
}
```

Исходная ошибка никогда не теряется: из `Result.Err` можно добраться и до `*net.DNSError`, и до `syscall.Errno`.

## Отмена и обязанности вызывающего

**Вызывающий обязан либо дочитать канал результатов до конца, либо отменить контекст.**

Канал результатов небуферизованный. Если перестать читать и не отменить контекст, воркеры навсегда останутся заблокированными на отправке. Отличить «читаю медленно» от «бросил читать» библиотека не может — об этом можно сообщить только отменой.

Медленное чтение проблемой не является: небуферизованный канал создаёт обратное давление, воркеры притормаживают, память не растёт, результаты не теряются.

Правильная форма:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
```

После отмены сканер перестаёт выдавать новые задания, доводит уже начатые проверки и закрывает канал. Результаты, оказавшиеся в работе на момент отмены, могут прийти со статусом `canceled`, если вызывающий продолжает читать; иначе они отбрасываются.

**Порядок результатов не определён** — их выдаёт пул воркеров.

## Принятые решения по краевым случаям

| Случай | Решение | Обоснование |
|---|---|---|
| Порт `0` | Ошибка `ErrInvalidPort` | Порт 0 служебный: при привязке означает «выдай любой свободный». Подключиться к нему нельзя |
| Порт больше 65535 | Ошибка `ErrInvalidPort` | Внутри порт хранится как `uint16`, невалидное значение невыразимо по построению |
| Обратный диапазон `1000-1` | Ошибка `ErrInvalidRange` | Почти всегда опечатка. Молчаливый обмен границ скрыл бы баг вызывающего |
| Повторяющиеся порты | Дедупликация | Проверять один порт дважды бессмысленно |
| Пустой список портов | Ошибка `ErrNoPorts` | Пустой скан — результат ошибки в вызывающем коде |
| Пустой хост | Ошибка `ErrInvalidTarget` | Синхронно, до старта работы |
| Некорректный адрес | Трактуется как DNS-имя | Если имя не разрешится, придёт результат со статусом `error` |
| Неразрешимое DNS-имя | Один результат со статусом `error` | Одна битая цель не должна прерывать скан остальных |
| Повторяющиеся цели | Дедупликация после обрезки пробелов | Экономия дескрипторов, предсказуемое число результатов |
| Одно имя → несколько IP | Сканируются все адреса | `Host` хранит имя, `IP` — конкретный адрес |
| Отмена контекста | Новые задания не выдаются, начатые доводятся | Бросать установленные соединения хуже, чем доработать |

## Ограничения

Не реализовано и реализовано не будет:

- SYN-сканирование, UDP-сканирование, raw-сокеты;
- определение операционной системы, обход межсетевых экранов;
- поддержка CIDR (в задании помечена как необязательная).

Практические ограничения:

- Каждая проверка занимает файловый дескриптор и локальный эфемерный порт. Конкурентность выше системного лимита (`ulimit -n` на unix) даёт ошибки, описывающие локальную машину, а не сканируемый узел.
- Разрешение DNS выполняется последовательно, по одной цели за раз.

## Зависимости

Кроме стандартной библиотеки используется одна:

| Пакет | Где | Обоснование |
|---|---|---|
| `go.uber.org/goleak` | Только в тестах | Детектор утечек горутин. Ручная реализация потребовала бы разбора `runtime.Stack` с фильтрацией служебных горутин — около сотни строк хрупкого кода. Не попадает в сборку приложений, подключающих пакет |

Коды ошибок Winsock объявлены в `classify_windows.go` собственными константами, чтобы не тянуть `golang.org/x/sys` ради девяти чисел. Значения зафиксированы тестом `TestWinsockCodesMatchMicrosoftValues`.

## Тесты

```bash
go test -race ./...
```

Дополнительно:

```bash
go test -short ./...                                  # без сетевых ожиданий
go test -fuzz=FuzzParsePorts -fuzztime=60s ./...      # фаззинг парсера портов
go test -bench=. -benchmem -run='^$' ./...            # бенчмарки
golangci-lint run                                     # линтеры
```

Что покрыто:

- юнит-тесты на разбор портов и целей, опции, классификацию ошибок;
- интеграционные тесты на настоящих сокетах через `net.Listen("tcp", "127.0.0.1:0")`, включая IPv6;
- нагрузочные тесты на гонки, отмену в разные моменты и переиспользование сканера;
- проверка утечек горутин через `goleak` в каждом конкурентном тесте;
- фаззинг `ParsePorts` с проверкой инвариантов;
- CI на Linux, macOS и Windows, на Go 1.25.5 и на текущей стабильной версии.

Сетевой слой и резолвер в юнит-тестах подменяются через внутренние интерфейсы, поэтому тесты не зависят от наличия интернета.

## Лицензия

MIT, см. [LICENSE](LICENSE).

---

<a id="english"></a>

# tcpscan

[Русский](#tcpscan) · **English**

A Go package that checks the reachability of TCP ports on one or more hosts.

It performs a plain TCP connect scan using the standard library only: the package asks the operating system to establish a connection and closes it immediately. No raw sockets and no elevated privileges are required.

The package is meant to be embedded into other applications and depends on no CLI, HTTP framework, database or message broker.

## Requirements

Go 1.25.5 or newer.

## Installation

```bash
go get github.com/we1lman/tcpscan
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/we1lman/tcpscan"
)

func main() {
	scanner, err := tcpscan.New(
		tcpscan.WithConcurrency(100),
		tcpscan.WithConnectTimeout(500*time.Millisecond),
	)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results, err := scanner.Scan(
		ctx,
		[]string{"127.0.0.1", "example.com"},
		tcpscan.Range(1, 1024),
	)
	if err != nil {
		panic(err)
	}

	for r := range results {
		if r.State != tcpscan.StateOpen {
			continue
		}
		fmt.Printf("%s (%s):%d open in %s\n", r.Host, r.IP, r.Port, r.Duration)
	}
}
```

## Public API

The public API has four parts: the scanner, the options, the port sets and the results.

### Scanner

```go
func New(opts ...Option) (*Scanner, error)
func (s *Scanner) Scan(ctx context.Context, hosts []string, ports PortSet) (<-chan Result, error)
```

`New` builds a scanner by applying options on top of the defaults. The first option that fails aborts construction and `nil` is returned. A `nil` option is skipped.

A `Scanner` is immutable after construction. It can be reused, and `Scan` may be called from several goroutines at once.

`Scan` checks every port of the set on every target and streams the results.

Errors come in two kinds:

| Kind | How it arrives | Examples |
|---|---|---|
| Invalid input | As a returned value, synchronously | empty target list, port 0, reversed range |
| Failure during the scan | As a result on the channel | port closed, timeout, name not resolved |

The input is validated before any work starts: if `Scan` returns an error, no channel is created and no connection is opened.

### Options

```go
func WithConcurrency(n int) Option
func WithConnectTimeout(d time.Duration) Option
```

| Option | Accepted values | Default |
|---|---|---|
| `WithConcurrency` | 1–65535 | 100 |
| `WithConnectTimeout` | greater than zero | 2s |

`WithConcurrency` caps how many connection attempts may be in flight at once. `WithConnectTimeout` limits **a single** check; an overall deadline for the whole scan belongs on the context.

The `Option` type is exported, but its implementation is closed: options cannot be defined outside the package, so every option reaching `New` has been validated.

### Port sets

```go
func Ports(ports ...int) PortSet
func Range(from, to int) PortSet
func Union(sets ...PortSet) PortSet
func ParsePorts(spec string) PortSet

func (s PortSet) Err() error
func (s PortSet) Len() int
```

`PortSet` is a sorted set of ports without duplicates. It is immutable and safe to share between concurrent scans.

```go
tcpscan.Ports(22, 80, 443)
tcpscan.Range(1, 1024)
tcpscan.Union(tcpscan.Range(20, 25), tcpscan.Ports(80, 443))
tcpscan.ParsePorts("22,80,443,8000-8100")
```

**Validation is deferred.** A constructor never fails on the spot, it stores the problem inside the set instead. This keeps sets buildable directly in the arguments of `Scan`, as shown above. The error is reported by `Err()` or by `Scan` itself.

The format accepted by `ParsePorts` is a comma separated list of ports and ranges: `80`, `22,80,443`, `1-1024`, `20-25,80,8000-8100`. Space around elements and around the dash is ignored.

### Results

```go
type Result struct {
	Host     string
	IP       net.IP
	Port     uint16
	State    State
	Duration time.Duration
	Err      error
}
```

`Host` is the target as it was passed by the caller, with surrounding space removed. `IP` is the address that was actually dialled; it differs from `Host` when the target was a DNS name.

When a target cannot be resolved, a single result is produced for it: `Host` and `Err` are set, while `IP`, `Port` and `Duration` keep their zero values.

### States

```go
const (
	StateUnknown State = iota
	StateOpen
	StateClosed
	StateTimeout
	StateUnreachable
	StateCanceled
	StateError
)
```

| State | Meaning |
|---|---|
| `open` | the TCP connection was established |
| `closed` | the host answered but refused: nothing is listening |
| `timeout` | the host stayed silent longer than the connect timeout |
| `unreachable` | there is no route to the host or to the network |
| `canceled` | the context was cancelled before the check finished |
| `error` | any other failure, including a failed name lookup |
| `unknown` | the zero value, the result was never filled in |

States are derived **from typed errors only** — through `errors.Is` and `errors.As` over `context.Canceled`, `context.DeadlineExceeded`, the `net.Error` interface, the `*net.DNSError` type and `syscall.Errno` system codes. No error text is ever compared; this is locked down by `TestClassifyIgnoresErrorText`.

The key distinction: `closed` is an **answer** from the host, `timeout` is **silence**. Silence allows no firm conclusion.

### Errors

```go
var (
	ErrNoPorts       = errors.New("tcpscan: no ports specified")
	ErrInvalidPort   = errors.New("tcpscan: invalid port")
	ErrInvalidRange  = errors.New("tcpscan: invalid port range")
	ErrNoTargets     = errors.New("tcpscan: no targets specified")
	ErrInvalidTarget = errors.New("tcpscan: invalid target")
	ErrInvalidOption = errors.New("tcpscan: invalid option")
)

type TargetError struct {
	Input string
	Err   error
}
```

All errors are matched with `errors.Is`. `TargetError` additionally reports **which** target failed and implements `Unwrap`, so both forms work at the same time:

```go
if errors.Is(err, tcpscan.ErrInvalidTarget) {
	// what happened
}

var targetErr *tcpscan.TargetError
if errors.As(err, &targetErr) {
	// which target it happened to
	log.Printf("bad target: %s", targetErr.Input)
}
```

The original error is never lost: `Result.Err` still leads to `*net.DNSError` and to `syscall.Errno`.

## Cancellation and the caller's obligations

**The caller must either read the result channel until it is closed or cancel the context.**

The result channel is unbuffered. Abandoning it without cancelling the context leaves the workers blocked on send forever. The library cannot tell "reading slowly" from "stopped reading" — cancellation is the only way to say so.

Slow reading is not a problem: an unbuffered channel provides back pressure, the workers slow down, memory does not grow and no result is lost.

The idiomatic form:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
```

Once cancelled, the scanner stops handing out new work, lets the checks already in flight finish and closes the channel. Results in flight at that moment may arrive with `canceled` if the caller keeps reading; otherwise they are dropped.

**The order of results is undefined** — they come from a pool of workers.

## Decisions on edge cases

| Case | Decision | Rationale |
|---|---|---|
| Port `0` | `ErrInvalidPort` | Port 0 is reserved: on bind it means "give me any free port". It cannot be connected to |
| Port above 65535 | `ErrInvalidPort` | Ports are stored as `uint16`, so an invalid value is not expressible |
| Reversed range `1000-1` | `ErrInvalidRange` | Almost always a typo. Silently swapping the bounds would hide the caller's bug |
| Duplicate ports | Deduplicated | Checking the same port twice is pointless |
| Empty port list | `ErrNoPorts` | An empty scan is the result of a bug in the calling code |
| Empty host | `ErrInvalidTarget` | Reported synchronously, before any work starts |
| Malformed address | Treated as a DNS name | If it does not resolve, a result with `error` is produced |
| Unresolvable DNS name | One result with `error` | One broken target must not abort the rest of the scan |
| Duplicate targets | Deduplicated after trimming | Saves descriptors, keeps the result count predictable |
| One name, several IPs | Every address is scanned | `Host` keeps the name, `IP` holds the address |
| Context cancelled | No new work, checks in flight finish | Dropping established connections is worse than finishing them |

## Limitations

Out of scope by design:

- SYN scanning, UDP scanning, raw sockets;
- OS detection, firewall evasion;
- CIDR support (marked optional in the assignment).

Practical limits:

- Every check holds a file descriptor and a local ephemeral port. Concurrency above the system limit (`ulimit -n` on unix) produces errors describing the local machine rather than the scanned host.
- DNS resolution is sequential, one target at a time.

## Dependencies

One dependency beyond the standard library:

| Package | Where | Rationale |
|---|---|---|
| `go.uber.org/goleak` | Tests only | Goroutine leak detector. A hand written version would need to parse `runtime.Stack` and filter runtime goroutines — about a hundred lines of fragile code. It does not reach applications that import this package |

Winsock error codes are declared as local constants in `classify_windows.go` instead of pulling in `golang.org/x/sys` for nine numbers. The values are pinned by `TestWinsockCodesMatchMicrosoftValues`.

## Tests

```bash
go test -race ./...
```

Additionally:

```bash
go test -short ./...                                  # skips network waits
go test -fuzz=FuzzParsePorts -fuzztime=60s ./...      # fuzzing the port parser
go test -bench=. -benchmem -run='^$' ./...            # benchmarks
golangci-lint run                                     # linters
```

What is covered:

- unit tests for port and target parsing, options and error classification;
- integration tests over real sockets via `net.Listen("tcp", "127.0.0.1:0")`, including IPv6;
- stress tests for races, cancellation at different moments and scanner reuse;
- goroutine leak checks with `goleak` in every concurrent test;
- fuzzing of `ParsePorts` with invariant checks;
- CI on Linux, macOS and Windows, on Go 1.25.5 and on the current stable release.

The network layer and the resolver are replaced through internal interfaces in unit tests, so the tests do not depend on internet access.

## License

MIT, see [LICENSE](LICENSE).
