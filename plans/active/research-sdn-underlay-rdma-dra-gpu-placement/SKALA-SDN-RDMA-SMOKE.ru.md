# RDMA И GPUDirect RDMA На RED OS 8

## Назначение

Документ покрывает:

- `RDMA` на узле;
- `GPUDirect RDMA` на узле;
- `RDMA` в поде;
- `GPUDirect RDMA` в поде;
- сравнения `TCP` и `RDMA` на одном и том же прямом канале.

Рассматривается `RED OS 8` и Mellanox `RoCE v2`. Ниже собраны
последовательные шаги и проверенные команды для подготовки узла и
последующих проверок.

## Область применимости

Проверенная базовая конфигурация:

- `RED OS 8.0.2`
- kernel `6.12.37-1.red80.x86_64`
- Mellanox 100GbE NIC с `RoCE v2`
- NVIDIA `Tesla V100-PCIE-32GB`
- `MLNX_OFED_SRC-24.10-3.2.5.0`
- `perftest` с поддержкой CUDA

## Как выглядел проверенный стенд

Ниже приведены конкретные значения того стенда, на котором проверялись
команды из этого документа. На другой установке имена нод, IP-адреса,
интерфейсы и `GID index` могут отличаться, но порядок проверки остаётся тем
же.

### Узлы

| Роль   | Имя узла                  | Прямой IP для проверки на узле |
| ------ | ------------------------- | ------------------------------ |
| server | `k8s-dvp-w1-gpu.apiac.ru` | `192.168.3.60`                 |
| client | `k8s-dvp-w3-gpu.apiac.ru` | `192.168.3.32`                 |

### Пара `GPU/NIC`, которая дала лучший результат на узле

| Узел | RDMA device | GPU    | PCI BUS ID         |
| ---- | ----------- | ------ | ------------------ |
| `w1` | `mlx5_0`    | `GPU1` | `00000000:C2:00.0` |
| `w3` | `mlx5_0`    | `GPU0` | `00000000:01:00.0` |

### Три `GPUDirect` pod на стенде

На проверенном стенде оставлены три живых pod с одновременным `GPU + RDMA`
path.

| Назначение                     | Pod                   | Узел | GPU                | Интерфейс     | Что используется                              |
| ------------------------------ | --------------------- | ---- | ------------------ | ------------- | --------------------------------------------- |
| Межузловой benchmark server    | `gpudirect-pod-w1-c2` | `w1` | `00000000:C2:00.0` | `enp193s0np0` | plain `RDMA`, `GPUDirect RDMA`, `TCP`         |
| Межузловой benchmark client    | `gpudirect-pod-w3-01` | `w3` | `00000000:01:00.0` | `enp2s0np0`   | plain `RDMA`, `GPUDirect RDMA`, `TCP`         |
| Дополнительный `GPUDirect` pod | `gpudirect-pod-w1-80` | `w1` | `00000000:81:00.0` | `enp130s0np0` | локальная проверка `GPU + verbs + direct NIC` |

Для всех межузловых benchmark в этом документе используется одна и та же пара:

- `gpudirect-pod-w1-c2` на `w1`;
- `gpudirect-pod-w3-01` на `w3`.

Третий pod `gpudirect-pod-w1-80` нужен как дополнительная живая surface на
spare `V100 + RDMA` паре. На текущем стенде у `rdma-w1-pair80` нет второй
стороны на `w3`, поэтому throughput/latency сравнения снимаются не на нём, а
на основной межузловой паре `w1-c2 <-> w3-01`.

### Прямая сеть, которая использовалась для benchmark в pod

| Пара pod                                      | IP на `w1`        | IP на `w3`        | `GID index` |
| --------------------------------------------- | ----------------- | ----------------- | ----------- |
| `gpudirect-pod-w1-c2 <-> gpudirect-pod-w3-01` | `172.31.130.1/30` | `172.31.130.2/30` | `3`         |

На таком же стенде эти значения можно подставлять напрямую. На другом стенде
сначала определяются свои IP-адреса, интерфейсы и `GID index`, а затем уже
запускаются тесты.

## С чего начинать на пустой ноде

Для чистого узла базовая конфигурация выглядит так.

### Целевая версия ядра

Проверенная комбинация:

- `RED OS 8.0.2`
- kernel `6.12.37-1.red80.x86_64`

Ядро устанавливается так:

```bash
dnf install -y \
  kernel-lt-6.12.37-1.red80.x86_64 \
  kernel-lt-devel-6.12.37-1.red80.x86_64 \
  kernel-lt-headers-6.12.37-1.red80.x86_64 \
  gcc make elfutils-libelf-devel

grubby --set-default /boot/vmlinuz-6.12.37-1.red80.x86_64
reboot
```

После первой перезагрузки добавляются параметры:

```bash
grubby --update-kernel=ALL --args="intel_iommu=on iommu=pt"
reboot
```

После второй перезагрузки состояние проверяется так:

```bash
uname -r
cat /proc/cmdline
```

Исправное состояние:

- `uname -r` возвращает `6.12.37-1.red80.x86_64`;
- в `cmdline` видны `intel_iommu=on iommu=pt`.

### Проверенные артефакты

Для такой же ноды на `RED OS 8` использовались использовались [следующие артефакты:](https://disk.360.yandex.ru/d/7LWtFzDegnTQjA):

| Что ставилось                                            | Точный артефакт                                                                                |
| -------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| Архив OFED                                               | `/root/MLNX_OFED_SRC-24.10-3.2.5.0.tgz`                                                        |
| Распакованный каталог OFED                               | `/root/MLNX_OFED_SRC-24.10-3.2.5.0`                                                            |
| Основной RPM OFED stack                                  | `/root/MLNX_OFED_SRC-24.10-3.2.5.0/RPMS/redos-release-8.0.2-37.red80/x86_64/*.rpm`             |
| Пересобранный `perftest` с поддержкой CUDA               | `/root/rpmbuild-perftest-cuda/RPMS/x86_64/perftest-24.10.0-0.95.g370212b.2410325.x86_64.rpm`   |
| Пересобранный `mpitests_openmpi`                         | `/root/rpmbuild-mpitests-fixed/RPMS/x86_64/mpitests_openmpi-3.2.24-2ffc2d6.2410068.x86_64.rpm` |
| Заголовок CUDA, использованный при пересборке `perftest` | `/root/w1-cuda.h`                                                                              |
| Репозиторий `nvidia-container-toolkit`                   | `https://nvidia.github.io/libnvidia-container/stable/rpm/nvidia-container-toolkit.repo`        |

### Какие пакеты должны оказаться на узле

После настройки на узле были установлены такие версии:

```bash
kernel-lt            6.12.37-1.red80
kernel-lt-devel      6.12.37-1.red80
ofed-scripts         24.10-OFED.24.10.3.2.5
rdma-core            2410mlnx54-1.2410068
ucx                  1.18.0-1.2410068
openmpi              4.1.7rc1-1.2410325
perftest             24.10.0-0.95.g370212b.2410325
mpitests_openmpi     3.2.24-2ffc2d6.2410068
```

### Рабочая последовательность на пустой ноде

Рабочая последовательность выглядит так:

1. Устанавливается целевое ядро `6.12.37-1.red80.x86_64`, `kernel-lt-devel`
   и `kernel-lt-headers`, затем узел перезагружается.
2. Добавляются `intel_iommu=on iommu=pt`, затем узел ещё раз
   перезагружается.
3. Ставятся базовые пакеты для сборки и RPM.
4. На узел кладётся архив `MLNX_OFED_SRC-24.10-3.2.5.0.tgz` и
   распаковывается в `/root/MLNX_OFED_SRC-24.10-3.2.5.0`.
5. Из этого каталога ставится `MLNX OFED`.
6. NVIDIA driver переустанавливается поверх уже установленного OFED.
7. Из официального RPM-репозитория ставится `nvidia-container-toolkit`.
8. При необходимости штатный `perftest` заменяется на пересобранный пакет с
   поддержкой CUDA.
9. Если нужен полноценный набор MPI-тестов, ставится пересобранный
   `mpitests_openmpi`.

Для `V100` основной путь `GPUDirect RDMA` в этом документе идёт через
`nvidia-peermem`, а не через `DMA-BUF`.

Причина:

- на проверенном стенде `GPUDirect RDMA` с `V100` был подтверждён через
  `nvidia-peermem`;
- `--use_cuda_dmabuf` виден в `perftest`, но не считается рабочим путём для
  этой базовой конфигурации;
- для `V100` исправное состояние на узле подтверждают:
  - успешный `modprobe nvidia-peermem`;
  - `perftest` с поддержкой CUDA;
  - успешный межузловой тест с `--use_cuda*`.

## Общая последовательность проверки

До запуска команд полезно держать в голове такую цепочку:

1. Подготовить ядро и включить `intel_iommu=on iommu=pt`.
2. Установить `MLNX OFED`.
3. Переустановить NVIDIA driver поверх уже установленного OFED и поднять
   `nvidia-peermem`.
4. Убедиться, что `perftest` собран с CUDA-поддержкой.
5. На узле сначала доказать сам факт `GPU memory -> RDMA`, а потом снять
   лучший скоростной профиль.
6. После этого переходить к проверкам в pod.
7. В pod сначала проверить обычный `RDMA`, затем `GPUDirect RDMA`.
8. В конце отдельно снять сравнение `TCP` и `RDMA` на том же прямом канале.

## Итоговое состояние

После выполнения инструкции итоговое состояние выглядит так:

- `RDMA` на узле работает:
  - `ibv_devinfo` показывает `PORT_ACTIVE`;
  - `active_mtu=4096`;
  - `link_layer=Ethernet`;
- предварительные условия для `GPUDirect RDMA` на узле выполнены:
  - `nvidia-smi` работает;
  - `nvidia-peermem` загружается;
  - `ib_write_bw --help` показывает `--use_cuda*`;
- есть рабочая межузловая проверка:
  - межнодовый `ib_write_bw --use_cuda=...` проходит;
- есть проверка скорости:
  - `ib_read_bw --use_cuda_bus_id=...` выходит на уровень, близкий к полной
    пропускной способности линии;
- `RDMA` в поде работает:
  - в pod есть прямой сетевой интерфейс и `/dev/infiniband/uverbs0`;
  - verbs-тест проходит внутри pod;
- `GPUDirect RDMA` в поде работает:
  - один и тот же pod одновременно получает GPU и RDMA-устройство;
  - межpodовый `ib_write_bw --use_cuda=...` проходит;
- есть понятное сравнение `TCP` против `RDMA` для команды эксплуатации.

## Ожидаемые метрики

Это не SLA и не гарантия для любого железа. Ниже приведены ориентирные
значения, полученные на проверенном стенде.

### Метрики на узле

| Сценарий                                | Команда                                 | Ожидаемый уровень        |
| --------------------------------------- | --------------------------------------- | ------------------------ |
| Базовый результат по сети               | `ib_write_bw`                           | около `86 Gb/sec`        |
| Проверка работы из памяти GPU           | `ib_write_bw --use_cuda=...`            | `~56-57 Gb/sec` или выше |
| Лучшая проверенная конфигурация на узле | `ib_read_bw --use_cuda_bus_id=... -q 8` | около `95 Gb/sec`        |

### Метрики в pod

| Сценарий                                                 | Команда                                 | Ожидаемый уровень   |
| -------------------------------------------------------- | --------------------------------------- | ------------------- |
| TCP без дополнительной настройки, пропускная способность | `iperf3 -P 4 -t 10`                     | `~15.7-23.5 Gb/sec` |
| TCP без дополнительной настройки, задержка               | `sockperf ping-pong --tcp -m 64`        | `~37.48 usec`       |
| RDMA, пропускная способность                             | `ib_read_bw -q 8`                       | `~97.30 Gb/sec`     |
| RDMA, задержка                                           | `ib_send_lat -s 64`                     | `~2.95 usec`        |
| `GPUDirect RDMA` в pod, functional proof                 | `ib_write_bw --use_cuda=...`            | `~56.46 Gb/sec`     |
| `GPUDirect RDMA` в pod, задержка                         | `ib_send_lat --use_cuda=... -s 64`      | `~4.57 usec`        |
| `GPUDirect RDMA` в pod, лучший скоростной профиль        | `ib_read_bw --use_cuda_bus_id=... -q 8` | `~95.52 Gb/sec`     |

Как читать эти цифры:

- `97.30 Gb/sec` для `RDMA` и `15.7-23.5 Gb/sec` для `TCP` здесь не надо читать как
  "теоретический максимум RDMA против теоретического максимума TCP". Это
  практический базовый замер в поде на одном прямом канале, но с разной
  степенью настройки:
  - `RDMA` запускался в уже подобранном профиле `ib_read_bw -q 8 -m 4096`;
  - `TCP` мерился как простой базовый тест через `iperf3 -P 4 -t 10`, без
    отдельной настройки сокетов и ядра и без заранее оформленной проверки
    Linux `MTU` и аппаратных разгрузок;
- задержка на небольших сообщениях в этих же условиях падает примерно с
  `37.48 usec` до `2.95 usec`;
- для `V100` при проверке `GPUDirect RDMA` важнее сам факт успешной работы,
  чем абсолютное значение `ib_write_bw`, потому что этот тест подтверждает
  путь `GPU memory -> RDMA`, а не наилучший скоростной профиль.

Если `TCP` и `RDMA` нужно сравнить максимально аккуратно, перед замером
отдельно фиксируются:

- `MTU` сетевого интерфейса внутри pod:

```bash
ip -d link show dev <iface>
```

- аппаратные разгрузки:

```bash
ethtool -k <iface>
```

- более нагруженный `TCP`-тест, например:

```bash
iperf3 -c <peer_ip> -B <local_ip> -P 8 -t 30 -w 4M
iperf3 -c <peer_ip> -B <local_ip> -P 16 -t 30 -w 4M
```

Если после этого `TCP` всё ещё остаётся далеко ниже полной пропускной
способности линии, это уже не просто вопрос методики, а признак упора в CPU
или в стек TCP-сокетов в текущей среде выполнения pod.

## Шаг 1. Подготовка хоста

### 1.1. Требования к хосту и виртуализации

На bare metal:

- Mellanox NIC должен быть виден в PCI;
- GPU должен быть виден в PCI;
- `IOMMU` не должен мешать `1:1 passthrough`.

На VM:

- для гостевой системы нужен `vIOMMU`;
- рекомендуемые параметры ядра гостевой системы:

```bash
intel_iommu=on iommu=pt
```

Проверка:

```bash
readlink -f /sys/bus/pci/devices/0000:02:00.0/iommu_group
```

Если `iommu_group` не виден и RDMA device пробрасывается как PCI device,
дальнейшая подготовка pod может завершаться с ошибкой.

### 1.2. Пакеты для сборки

Минимально достаточный набор пакетов для `RED OS 8`:

```bash
dnf install -y \
  gcc gcc-c++ make rpm-build \
  autoconf automake libtool cmake \
  libnl3-devel python3-devel systemd-devel \
  python3-Cython python3-docutils pandoc \
  pciutils-devel
```

Если пакеты уже установлены, повторная установка не повредит.

### 1.3. Установка Mellanox OFED

Исходники на проверенной базовой конфигурации:

```bash
export OFED=/root/MLNX_OFED_SRC-24.10-3.2.5.0
```

Команда установки:

```bash
cd "$OFED"
./install.pl \
  --all \
  --distro rhel8.10 \
  --without-depcheck \
  --force \
  --kernel "$(uname -r)" \
  --kernel-sources "/lib/modules/$(uname -r)/build"
```

После завершения установки узел перезагружается, а базовые пакеты
проверяются так:

```bash
reboot
rpm -q ofed-scripts rdma-core ucx openmpi perftest
```

На проверенном стенде после установки OFED на `w3` были такие версии:

```text
ofed-scripts-24.10-OFED.24.10.3.2.5.x86_64
rdma-core-2410mlnx54-1.2410068.x86_64
ucx-1.18.0-1.2410068.x86_64
openmpi-4.1.7rc1-1.2410325.x86_64
perftest-24.10.0-0.95.g370212b.2410325.x86_64
```

Практически важно следующее:

- на `RED OS 8` установка OFED из исходного комплекта практичнее, чем попытка
  обойтись только пакетами дистрибутива;
- пользовательские утилиты могут спотыкаться о упаковку и зависимости, но для
  `RDMA + GPUDirect` критичны именно:
  - `rdma-core` / verbs stack;
  - Mellanox kernel stack;
  - `perftest`;
  - `openmpi`, если нужен MPI-тест.

### 1.4. Переустановка NVIDIA driver после OFED

Для `V100` это ключевой шаг.

Если NVIDIA driver был поставлен до OFED, `nvidia-peermem` может оказаться
собранным против старого состояния RDMA symbols и не загрузиться.

После установки OFED порядок такой:

1. повторно запускается installer NVIDIA driver;
2. проверяется, что пересобрался `nvidia-peermem.ko`;
3. включается автозагрузка:

```bash
sh ./NVIDIA-Linux-x86_64-580.76.05.run \
  --skip-module-load \
  --no-dkms \
  --kernel-module-type=proprietary

printf 'nvidia-peermem\n' >/etc/modules-load.d/nvidia-peermem.conf
modprobe nvidia-peermem
```

Проверка выглядит так:

```bash
nvidia-smi
lsmod | grep nvidia_peermem
modinfo nvidia-peermem | sed -n '1,12p'
```

На исправленном `w3` это выглядело так:

```text
Driver Version: 580.76.05
CUDA Version: 13.0
...
nvidia_peermem        16384  0
license:        Dual BSD/GPL
version:        580.76.05
```

### 1.5. Проверка `perftest` с поддержкой CUDA

Если штатный `perftest` уже показывает `--use_cuda`, достаточно проверки.
Если нет, пакет заменяется на пересобранный RPM с поддержкой CUDA.

На проверенном стенде для `w3` использовался такой пакет:

```bash
rpm -Uvh --replacepkgs --nodeps \
  /root/rpmbuild-perftest-cuda/RPMS/x86_64/perftest-24.10.0-0.95.g370212b.2410325.x86_64.rpm
```

Проверка выполняется так:

```bash
ib_write_bw --help 2>&1 | egrep -i 'use_cuda|cuda_bus_id|cuda_dmabuf'
```

Минимально ожидаются:

- `--use_cuda`
- `--use_cuda_bus_id`

Если их нет, текущий `perftest` не годится для проверки `GPUDirect RDMA`.

Пример исправного вывода:

```text
  --use_cuda=<gpu index>      use CUDA memory
  --use_cuda_bus_id=<pci id>  use CUDA device by PCI bus ID
  --use_cuda_dmabuf           use CUDA DMA-BUF memory
```

### 1.6. Установка `nvidia-container-toolkit`

Если на ноде потом будут запускаться pod с GPU, удобнее поставить toolkit
сразу на этапе подготовки хоста:

```bash
curl -s -L \
  https://nvidia.github.io/libnvidia-container/stable/rpm/nvidia-container-toolkit.repo \
  >/etc/yum.repos.d/nvidia-container-toolkit.repo

dnf install -y \
  libnvidia-container1 \
  libnvidia-container-tools \
  nvidia-container-toolkit-base \
  nvidia-container-toolkit
```

Проверка:

```bash
rpm -q \
  libnvidia-container1 \
  libnvidia-container-tools \
  nvidia-container-toolkit-base \
  nvidia-container-toolkit
```

### 1.7. Установка пересобранного `mpitests_openmpi`

Если кроме `perftest` нужен ещё и комплект MPI-бенчмарков:

```bash
rpm -Uvh --replacepkgs \
  /root/rpmbuild-mpitests-fixed/RPMS/x86_64/mpitests_openmpi-3.2.24-2ffc2d6.2410068.x86_64.rpm

rpm -q mpitests_openmpi
```

## Шаг 2. Проверка исходных условий на узле

### 2.1. Проверка RDMA на узле

```bash
rdma link show
ibv_devices
ibv_devinfo -d mlx5_0 -v | egrep 'hca_id|state:|link_layer|active_mtu'
```

Что должно быть:

- `mlx5_0` или `mlx5_1` существует;
- `PORT_ACTIVE`;
- `link_layer: Ethernet`;
- `active_mtu: 4096`.

### 2.2. Проверка `GPUDirect RDMA` для `V100`

```bash
nvidia-smi -L
modinfo nvidia-peermem | sed -n '1,20p'
grep -E '\bib_register_peer_memory_client\b|\bib_unregister_peer_memory_client\b' /proc/kallsyms
modprobe nvidia-peermem
lsmod | egrep 'nvidia_peermem|mlx5_ib|ib_core'
ib_write_bw --help 2>&1 | egrep -i 'cuda|use_cuda'
```

Исправное состояние:

- `nvidia-smi` видит GPU;
- `modprobe nvidia-peermem` проходит;
- `lsmod` показывает `nvidia_peermem`;
- `ib_write_bw --help` показывает CUDA flags.

Критическая проблема:

- `modprobe nvidia-peermem` падает;
- в `ib_write_bw --help` нет `--use_cuda*`.

## Шаг 3. Проверка `GPUDirect RDMA` на узле

### 3.1. Определение прямого IP, `GID index` и связки `GPU/NIC`

Сначала фиксируются:

```bash
ip -br addr
ibv_devinfo -d mlx5_0 -v | sed -n '/GID\\[/,/^$/p'
nvidia-smi --query-gpu=index,pci.bus_id,name --format=csv
nvidia-smi topo -m
```

При выборе параметров важно следующее:

- прямой IP хоста, а не адрес management/overlay сети;
- `RoCE v2 GID index` для нужного IP;
- GPU, которая по топологии PCIe ближе к `mlx5_0`.

На проверенном стенде для проверок на узле использовались:

- `w1`: IP `192.168.3.60`, `mlx5_0`, `GID index 1`, `GPU1`, `00000000:C2:00.0`;
- `w3`: IP `192.168.3.32`, `mlx5_0`, `GID index 1`, `GPU0`, `00000000:01:00.0`.

### 3.2. Проверка пути из памяти GPU

Server:

```bash
ib_write_bw -d mlx5_0 -x <gid_index> -m 4096 -F --report_gbits -D 5 --use_cuda=0
```

Client:

```bash
ib_write_bw -d mlx5_0 -x <gid_index> -m 4096 -F --report_gbits -D 5 --use_cuda=0 <peer_ip>
```

Исправный результат:

- `RC=0`;
- benchmark доходит до конца;
- в client output есть:
  - `initializing CUDA`
  - `cuMemAlloc()`

На проверенном стенде `ib_write_bw --use_cuda=0` давал
`~56-57 Gb/sec`.

Рабочий пример с тестового стенда:

Server на `w1`:

```bash
ib_write_bw -d mlx5_0 -x 1 -m 4096 -F --report_gbits -D 5 --use_cuda=0
```

Client на `w3`:

```bash
ib_write_bw -d mlx5_0 -x 1 -m 4096 -F --report_gbits -D 5 --use_cuda=0 192.168.3.60
```

Клиент в исправном случае печатает строки вида:

```text
initializing CUDA
Picking device No. 0
device name = [Tesla V100-PCIE-32GB]
cuMemAlloc() of a 131072 bytes GPU buffer
...
BW average[Gb/sec] 56.99
```

В обратном направлении на том же стенде результат был около `56.80 Gb/sec`.

### 3.3. Основной тест скорости

Для проверки скорости на этой базовой конфигурации удобнее использовать не
`ib_write_bw`, а `ib_read_bw` с точной привязкой `GPU/NIC`.

Server:

```bash
ib_read_bw \
  -d mlx5_0 -x <gid_index> -q 8 -m 4096 -F --report_gbits -D 10 \
  --use_cuda_bus_id=<server_gpu_bus_id>
```

Client:

```bash
ib_read_bw \
  -d mlx5_0 -x <gid_index> -q 8 -m 4096 -F --report_gbits -D 10 \
  --use_cuda_bus_id=<client_gpu_bus_id> \
  <peer_ip>
```

Для подобного стенда нормальным считается следующее:

- `90+ Gb/sec` в лучшем направлении;
- `~95 Gb/sec` на хорошем `GPU/NIC` mapping;
- возможна асимметрия, особенно если одна из нод виртуализована.

На проверенном стенде:

- лучшая проверенная конфигурация дала `95.10 Gb/sec`;
- в обратном направлении, когда на `w3` удалённым источником служила GPU,
  результат оставался около `61.7 Gb/sec`.

Лучший рабочий профиль с тестового стенда:

Server на `w1`:

```bash
/root/perftest-cuda-build-w1/perftest-24.10.0/ib_read_bw \
  -d mlx5_0 -x 1 -q 8 -m 4096 -F --report_gbits -D 10 \
  --use_cuda_bus_id=00000000:C2:00.0
```

Client на `w3`:

```bash
ib_read_bw \
  -d mlx5_0 -x 1 -q 8 -m 4096 -F --report_gbits -D 10 \
  --use_cuda_bus_id=00000000:01:00.0 \
  192.168.3.60
```

Клиентский вывод в лучшем направлении:

```text
#bytes     #iterations    BW peak[Gb/sec]    BW average[Gb/sec]
65536      14034          95.10              95.10
```

Если поменять роли местами и сделать `w3` удалённым источником GPU-памяти,
на том же стенде скорость оставалась около `61.7 Gb/sec`. Это надо учитывать
при интерпретации результатов в виртуализованной среде.

## Шаг 4. Минимальные требования со стороны Kubernetes

Этот раздел относится к сценарию, в котором `RDMA` и `GPUDirect RDMA`
довозятся до pod.

### 4.1. Требования к underlay для Mellanox

Для Mellanox в текущем `sdn` используем:

```json
"bindingMode": "DPDK"
```

Практический смысл этого выбора:

- device остаётся на `mlx5_core`;
- в pod приезжает обычный сетевой интерфейс;
- в pod приезжают устройства verbs;
- внутри pod работают `ibv_devinfo`, `ib_write_bw`, `ib_send_lat`.

Для обычной проверки verbs/RoCE здесь не нужен `VFIO-PCI`.

### 4.2. Минимальные требования к безопасности pod

Подтверждённый минимальный набор требований:

- без `hostPath`;
- без `privileged: true`;
- `IPC_LOCK` нужен для verbs benchmark;
- `NET_ADMIN` нужен только если руками назначаешь временный `/30`;
- `NET_RAW` нужен только если используешь `ping`.

Если среда сама выдаёт IP на прямой интерфейс, тестовый pod можно
сжимать до одного `IPC_LOCK`.

### 4.3. Базовый шаблон pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: rdma-tools
  namespace: rdma-test
  annotations:
    network.deckhouse.io/networks-spec: |
      [
        {
          "type": "UnderlayNetwork",
          "name": "REPLACE_UNDERLAY",
          "bindingMode": "DPDK"
        }
      ]
spec:
  restartPolicy: Never
  nodeSelector:
    kubernetes.io/hostname: REPLACE_NODE
  containers:
    - name: tools
      image: registry.example/rdma-validation-toolkit:0.2.0
      command: ["sleep", "infinity"]
      securityContext:
        runAsUser: 0
        allowPrivilegeEscalation: false
        seccompProfile:
          type: RuntimeDefault
        capabilities:
          add:
            - IPC_LOCK
            - NET_ADMIN
            - NET_RAW
```

Для проверки `GPUDirect RDMA` этот же pod дополнительно получает GPU.

## Шаг 5. Проверка `RDMA` в pod

### 5.1. Проверка появления RDMA device в pod

```bash
rdma-validate inventory
```

Либо вручную:

```bash
ip -br link
ls -l /dev/infiniband
ibv_devices
ibv_devinfo -d mlx5_0 -v | egrep 'hca_id|state:|link_layer|active_mtu'
```

Что должно быть:

- внутри pod есть прямой сетевой интерфейс;
- есть `/dev/infiniband/uverbs0`;
- `ibv_devinfo` показывает `PORT_ACTIVE`, `link_layer=Ethernet`,
  `active_mtu=4096`.

На проверенном стенде для обычного `RDMA` использовалась та же живая
`GPUDirect` пара:

- `gpudirect-pod-w1-c2` на `w1`, интерфейс `enp193s0np0`;
- `gpudirect-pod-w3-01` на `w3`, интерфейс `enp2s0np0`.

Это важно: plain `RDMA`, `GPUDirect RDMA` и `TCP` baseline ниже снимаются на
одних и тех же двух pod и на одном и том же прямом канале.

### 5.2. Назначение временных прямых IP-адресов

На практике адреса удобнее назначать сразу явными командами.

Команды с тестового стенда:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w1-c2 -- \
  ip addr replace 172.31.130.1/30 dev enp193s0np0
kubectl -n sdn-rdma-test exec gpudirect-pod-w1-c2 -- \
  ip link set enp193s0np0 up

kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  ip addr replace 172.31.130.2/30 dev enp2s0np0
kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  ip link set enp2s0np0 up
```

После этого состояние проще всего проверить так:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w1-c2 -- ip -4 -br addr
kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- ip -4 -br addr
```

То же самое через `rdma-validate`.

Server pod:

```bash
export DIRECT_IFACE=<iface>
export DIRECT_IP=172.31.130.1/30
rdma-validate bind-ip
```

Client pod:

```bash
export DIRECT_IFACE=<iface>
export DIRECT_IP=172.31.130.2/30
rdma-validate bind-ip
```

### 5.3. Проверка `GID index`

```bash
ibv_devinfo -d mlx5_0 -v | sed -n '/GID\\[/,/^$/p'
```

Не хардкодь `-x`, всегда смотри текущий `RoCE v2 GID index` после назначения
IP.

На проверенном стенде после назначения `/30` для прямой сети в pod использовался
`GID index 3`.

### 5.4. Основной тест RDMA в pod

На тестовом стенде основной запуск выглядел так.

Server:

```bash
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w1-c2 -- \
  bash -lc 'ib_read_bw -d mlx5_0 -x 3 -q 8 -m 4096 -F --report_gbits -D 5'
```

Client:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  bash -lc 'ib_read_bw -d mlx5_0 -x 3 -q 8 -m 4096 -F --report_gbits -D 5 172.31.130.1'
```

Именно в таком виде команда обычно и запускается вручную: сервер слушает без
адреса, клиент получает тот же набор флагов и в конце адрес второй стороны.

Ожидаемый уровень:

- `90+ Gb/sec`
- на проверенном стенде около `97.30 Gb/sec`

В исправном случае результат выглядит так:

```text
#bytes     #iterations    BW peak[Gb/sec]    BW average[Gb/sec]
65536      556761         0.00               97.30
```

Та же проверка через `rdma-validate`.

Server:

```bash
export RDMA_DEV=mlx5_0
export GID_INDEX=<gid_index>
export QPS=8
export MTU=4096
export SIZE=65536
rdma-validate rdma-bw-server
```

Client:

```bash
export RDMA_DEV=mlx5_0
export GID_INDEX=<gid_index>
export PEER_IP=172.31.130.1
export QPS=8
export MTU=4096
export SIZE=65536
rdma-validate rdma-bw-client
```

### 5.5. Задержка RDMA в pod

На тестовом стенде задержка проверялась такими прямыми командами.

Server:

```bash
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w1-c2 -- \
  bash -lc 'ib_send_lat -d mlx5_0 -x 3 -s 64 -n 2000 -F'
```

Client:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  bash -lc 'ib_send_lat -d mlx5_0 -x 3 -s 64 -n 2000 -F 172.31.130.1'
```

Ожидаемый уровень:

- единицы микросекунд
- на проверенном стенде около `2.95 usec`

На проверенном стенде итог выглядел так:

```text
 t_min[usec]  t_max[usec]  t_typical[usec]  t_avg[usec]  t_stdev[usec]
 2.86         64.89        2.90             2.95         0.41
```

Та же проверка через `rdma-validate`.

Server:

```bash
export RDMA_DEV=mlx5_0
export GID_INDEX=<gid_index>
export LAT_SIZE=64
rdma-validate rdma-lat-server
```

Client:

```bash
export RDMA_DEV=mlx5_0
export GID_INDEX=<gid_index>
export PEER_IP=172.31.130.1
export LAT_SIZE=64
rdma-validate rdma-lat-client
```

## Шаг 6. Проверка `GPUDirect RDMA` в pod

### 6.1. Выбор конкретной пары `GPU/NIC`

Внутри pod:

```bash
nvidia-smi --query-gpu=index,pci.bus_id,name --format=csv
nvidia-smi topo -m
```

Выбирается GPU, которая по топологии PCIe ближе к RDMA NIC.

### 6.2. Проверка того, что pod действительно получил GPU

```bash
nvidia-smi -L
ls -l /dev/infiniband
ib_write_bw --help | egrep -i 'use_cuda|cuda_bus_id|cuda_dmabuf'
```

Исправное состояние:

- GPU видна в `nvidia-smi`;
- verbs devices приехали;
- `ib_write_bw --help` показывает CUDA flags.

Если в pod штатного `perftest` с поддержкой CUDA нет, его можно собрать прямо внутри
тестового pod. На проверенном стенде использовалась такая последовательность:

```bash
apt-get update >/dev/null
apt-get install -y \
  git autoconf automake libtool pkg-config build-essential \
  libibverbs-dev librdmacm-dev libibumad-dev libpci-dev >/dev/null
git clone --depth 1 --branch 24.10.0-0.66 https://github.com/linux-rdma/perftest.git /tmp/perftest
cd /tmp/perftest
./autogen.sh >/tmp/perftest-autogen.log 2>&1
./configure CUDA_H_PATH=/usr/local/cuda/include/cuda.h >/tmp/perftest-configure.log 2>&1
make -j2 >/tmp/perftest-make.log 2>&1
./ib_write_bw --help | egrep -i 'use_cuda|cuda_dmabuf|cuda_bus_id'
```

На проверенном стенде в pod использовались:

- `gpudirect-pod-w1-c2`, GPU `00000000:C2:00.0`, прямой IP `172.31.130.1/30`;
- `gpudirect-pod-w3-01`, GPU `00000000:01:00.0`, прямой IP `172.31.130.2/30`;
- `gpudirect-pod-w1-80`, GPU `00000000:81:00.0`, direct interface
  `enp130s0np0`.

Быстрая локальная проверка третьего pod:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w1-80 -- \
  bash -lc 'nvidia-smi -L; ls -l /dev/infiniband; ip -br link; ib_write_bw --help | egrep -i "use_cuda|cuda_bus_id|cuda_dmabuf"'
```

На текущем стенде этот pod нужен как дополнительная живая `GPU + RDMA`
surface на spare `V100`. Его direct underlay `rdma-w1-pair80` не имеет второй
стороны на `w3`, поэтому межузловые benchmark ниже выполняются на паре
`gpudirect-pod-w1-c2 <-> gpudirect-pod-w3-01`.

### 6.3. Проверка `GPUDirect RDMA` в pod

На проверенном стенде окончательная проверка делалась на уже поднятых pod:

- `gpudirect-pod-w1-c2` на `k8s-dvp-w1-gpu.apiac.ru`;
- `gpudirect-pod-w3-01` на `k8s-dvp-w3-gpu.apiac.ru`.

Прямые адреса для этих pod:

- `gpudirect-pod-w1-c2/enp193s0np0`: `172.31.130.1/30`;
- `gpudirect-pod-w3-01/enp2s0np0`: `172.31.130.2/30`.

`RoCE v2 GID index` на обеих сторонах: `3`.

На этом же стенде использовались такие GPU:

- `w1`: `00000000:C2:00.0`;
- `w3`: `00000000:01:00.0`.

Если в pod `perftest` уже собран и команды вынесены в `PATH`, проверки ниже
можно запускать как есть. Если `perftest` собран вручную в каталоге
`/tmp/perftest` или `/opt/perftest-src`, вместо системного бинаря используй
полный путь к нему.

#### 6.3.1. Минимальная рабочая проверка: `ib_write_bw --use_cuda=0`

Этот тест нужен в первую очередь как доказательство самого пути
`GPU memory -> RDMA`.

Server на `w1`:

```bash
kubectl -n sdn-rdma-test exec -it gpudirect-pod-w1-c2 -- \
  ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 5 --use_cuda=0
```

Client на `w3`:

```bash
kubectl -n sdn-rdma-test exec -it gpudirect-pod-w3-01 -- \
  ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 5 --use_cuda=0 172.31.130.1
```

Исправный результат выглядит так:

- `RC=0`;
- в выводе есть:
  - `initializing CUDA`;
  - `device name = [Tesla V100-PCIE-32GB]`;
  - `cuMemAlloc()`.

Клиентский вывод на проверенном стенде:

```text
initializing CUDA
device name = [Tesla V100-PCIE-32GB]
cuMemAlloc() of a 131072 bytes GPU buffer
...
BW average[Gb/sec] 56.46
```

Более полный фрагмент того же вывода:

```text
RDMA_Write BW Test
Device         : mlx5_0
Link type      : Ethernet
GID index      : 3
...
local address:  GID ...172:31:130:02
remote address: GID ...172:31:130:01
...
#bytes     #iterations    BW peak[Gb/sec]    BW average[Gb/sec]
65536      323069         0.00               56.46
```

Для `V100` это считается достаточным functional proof. Здесь важен сам факт,
что тест идёт из памяти GPU и завершается без ошибок.

#### 6.3.2. Задержка из памяти GPU: `ib_send_lat --use_cuda=0`

Этот тест показывает latency на тех же двух pod и том же прямом канале.

Server на `w1`:

```bash
kubectl -n sdn-rdma-test exec -it gpudirect-pod-w1-c2 -- \
  ib_send_lat -d mlx5_0 -x 3 -s 64 -n 2000 -F --use_cuda=0
```

Client на `w3`:

```bash
kubectl -n sdn-rdma-test exec -it gpudirect-pod-w3-01 -- \
  ib_send_lat -d mlx5_0 -x 3 -s 64 -n 2000 -F --use_cuda=0 172.31.130.1
```

На проверенном стенде получилось:

```text
t_min[usec]            4.46
t_max[usec]           10.24
t_avg[usec]            4.57
99% percentile[usec]   6.32
99.9% percentile[usec] 9.27
```

Фрагмент полного вывода:

```text
Send Latency Test
Device         : mlx5_0
Link type      : Ethernet
GID index      : 3
...
#bytes #iterations    t_min[usec]    t_max[usec]    t_avg[usec]
64     2000           4.46           10.24          4.57
```

И здесь, как и в bandwidth-тесте, в выводе должны присутствовать
`initializing CUDA` и `cuMemAlloc()`.

#### 6.3.3. Лучший скоростной профиль: `ib_read_bw --use_cuda_bus_id ... -q 8`

Для `V100` именно этот тест на том же стенде дал near-line-rate профиль.
Здесь используется точная привязка к GPU по `PCI BUS ID`.

Server на `w1`:

```bash
kubectl -n sdn-rdma-test exec -it gpudirect-pod-w1-c2 -- \
  ib_read_bw -d mlx5_0 -x 3 -q 8 -m 4096 -F --report_gbits -D 5 \
  --use_cuda_bus_id=00000000:C2:00.0
```

Client на `w3`:

```bash
kubectl -n sdn-rdma-test exec -it gpudirect-pod-w3-01 -- \
  ib_read_bw -d mlx5_0 -x 3 -q 8 -m 4096 -F --report_gbits -D 5 \
  --use_cuda_bus_id=00000000:01:00.0 172.31.130.1
```

Клиентский вывод на проверенном стенде:

```text
Got PCIe address of: 00000000:01:00.0
...
BW average[Gb/sec] 95.52
```

Более полный фрагмент:

```text
RDMA_Read BW Test
Device           : mlx5_0
Number of qps    : 8
Link type        : Ethernet
GID index        : 3
...
#bytes     #iterations    BW peak[Gb/sec]    BW average[Gb/sec]
65536      546541         0.00               95.52
```

И здесь тоже обязательны признаки живого `GPU memory` path:

- `initializing CUDA`;
- `Picking GPU number 0` или аналогичное сообщение выбора устройства;
- `cuMemAlloc()`.

Практическая интерпретация для `V100` на этом стенде такая:

- `ib_write_bw --use_cuda=0` используется как основной functional proof;
- `ib_send_lat --use_cuda=0` показывает рабочую задержку на `GPU memory` path;
- `ib_read_bw --use_cuda_bus_id ... -q 8` используется как лучший скоростной
  профиль и на этом стенде даёт около `95.52 Gb/sec`.

Итог по финальной паре pod:

| Проверка                          | Pod server            | Pod client            | Результат         |
| --------------------------------- | --------------------- | --------------------- | ----------------- |
| `GPUDirect RDMA` functional proof | `gpudirect-pod-w1-c2` | `gpudirect-pod-w3-01` | `56.46 Gb/sec`    |
| `GPUDirect RDMA` latency          | `gpudirect-pod-w1-c2` | `gpudirect-pod-w3-01` | `t_avg 4.57 usec` |
| `GPUDirect RDMA` fastest profile  | `gpudirect-pod-w1-c2` | `gpudirect-pod-w3-01` | `95.52 Gb/sec`    |

## Шаг 7. Сравнение `TCP` и `RDMA` на одном канале

Для сравнения в этом разделе используется та же межузловая `GPUDirect` пара,
что и выше:

- `gpudirect-pod-w1-c2`, `172.31.130.1/30`, `enp193s0np0`;
- `gpudirect-pod-w3-01`, `172.31.130.2/30`, `enp2s0np0`.

То есть ниже сравниваются не разные pod, а один и тот же прямой канал в трёх
вариантах:

- обычный `TCP` по Ethernet;
- plain `RDMA` без `--use_cuda`;
- `GPUDirect RDMA` с `--use_cuda*`.

Если `iperf3` и `sockperf` в benchmark-pair ещё не стоят, перед `TCP`-тестами
их надо доустановить:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w1-c2 -- \
  bash -lc 'export DEBIAN_FRONTEND=noninteractive; apt-get update && apt-get install -y iperf3 sockperf'

kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  bash -lc 'export DEBIAN_FRONTEND=noninteractive; apt-get update && apt-get install -y iperf3 sockperf'
```

Если нужен образ, в котором всё уже есть из коробки, вместо ручной
доустановки удобнее использовать toolkit из шага `8`.

### 7.1. `TCP` без дополнительной настройки: пропускная способность

Server на `w1`:

```bash
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w1-c2 -- \
  bash -lc 'iperf3 -s -B 172.31.130.1 -1'
```

Client на `w3`:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  bash -lc 'iperf3 -c 172.31.130.1 -B 172.31.130.2 -P 4 -t 10'
```

В этом направлении на стенде получилось:

```text
[SUM]   0.00-10.00  sec  18.4 GBytes  15.8 Gbits/sec
```

Обратный прогон:

```bash
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w3-01 -- \
  bash -lc 'iperf3 -s -B 172.31.130.2 -1'

kubectl -n sdn-rdma-test exec gpudirect-pod-w1-c2 -- \
  bash -lc 'iperf3 -c 172.31.130.2 -B 172.31.130.1 -P 4 -t 10'
```

Во втором направлении на том же стенде получилось:

```text
[SUM]   0.00-10.00  sec  27.4 GBytes  23.6 Gbits/sec
```

Если нужен более строгий базовый замер, перед тестом выполняются:

```bash
ip -d link show dev <iface>
ethtool -k <iface>
```

А затем тест повторяется в более нагруженном профиле:

```bash
iperf3 -c <peer_ip> -B <local_ip> -P 8 -t 30 -w 4M
iperf3 -c <peer_ip> -B <local_ip> -P 16 -t 30 -w 4M
```

### 7.2. `TCP` без дополнительной настройки: задержка

Server на `w1`:

```bash
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w1-c2 -- \
  bash -lc 'sockperf server --tcp -i 172.31.130.1'
```

Client на `w3`:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  bash -lc 'sockperf ping-pong --tcp -i 172.31.130.1 --client_ip 172.31.130.2 -m 64 -t 10'
```

Ключевые цифры из результата:

```text
avg-latency=37.480 usec
p50-latency=36.310 usec
p99-latency=50.895 usec
p99.9-latency=112.102 usec
```

Полный RTT на том же стенде:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  bash -lc 'sockperf ping-pong --tcp -i 172.31.130.1 --client_ip 172.31.130.2 -m 64 -t 10 --full-rtt'
```

Итоговый средний RTT на стенде:

```text
avg-rtt=79.663 usec
```

### 7.3. Сводка

На проверенном стенде итоговая картина по той же самой паре pod была такой:

| Метрика                                                    | Результат          |
| ---------------------------------------------------------- | ------------------ |
| `TCP` без дополнительной настройки, пропускная способность | `15.8-23.6 Gb/sec` |
| `TCP` без дополнительной настройки, задержка               | `37.48 usec`       |
| `TCP` без дополнительной настройки, RTT                    | `79.663 usec`      |
| plain `RDMA`, пропускная способность                       | `97.30 Gb/sec`     |
| plain `RDMA`, задержка                                     | `2.95 usec`        |
| `GPUDirect RDMA`, functional proof                         | `56.46 Gb/sec`     |
| `GPUDirect RDMA`, задержка                                 | `4.57 usec`        |
| `GPUDirect RDMA`, лучший скоростной профиль                | `95.52 Gb/sec`     |

Практический вывод:

- plain `RDMA` на этом канале даёт примерно `4.1-6.2x` больше throughput,
  чем базовый `TCP`-тест без дополнительной настройки;
- по задержке plain `RDMA` на том же канале примерно в `12.7x` быстрее
  базового `TCP`;
- `GPUDirect RDMA` здесь оценивается двумя разными режимами:
  - `ib_write_bw --use_cuda=0` как functional proof самого пути
    `GPU memory -> RDMA`;
  - `ib_read_bw --use_cuda_bus_id ... -q 8` как лучший практический
    скоростной профиль;
- эти цифры подходят как ориентир для повторных проверок на таком же стенде,
  но не как универсальное ограничение для любого `TCP` или любой
  виртуализованной среды.

Если нужен полностью ручной вариант без `rdma-validate`, на текущем стенде
достаточно следующего порядка:

1. Проверить, что у `gpudirect-pod-w1-c2` и `gpudirect-pod-w3-01` уже стоят
   `172.31.130.1/30` и `172.31.130.2/30`.
2. Снять `TCP` throughput и latency командами из разделов `7.1` и `7.2`.
3. Снять plain `RDMA` throughput:

```bash
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w1-c2 -- \
  bash -lc 'ib_read_bw -d mlx5_0 -x 3 -q 8 -m 4096 -F --report_gbits -D 5'

kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  bash -lc 'ib_read_bw -d mlx5_0 -x 3 -q 8 -m 4096 -F --report_gbits -D 5 172.31.130.1'
```

На стенде это дало `97.30 Gb/sec`.

4. Снять plain `RDMA` latency:

```bash
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w1-c2 -- \
  bash -lc 'ib_send_lat -d mlx5_0 -x 3 -s 64 -n 2000 -F'

kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  bash -lc 'ib_send_lat -d mlx5_0 -x 3 -s 64 -n 2000 -F 172.31.130.1'
```

На стенде средняя задержка была `2.95 usec`.

## Шаг 8. Готовый контейнер с инструментами

Рядом с этим документом в репозитории лежит готовый набор файлов для сборки:

- [toolkit/rdma-validation-toolkit/Containerfile](./toolkit/rdma-validation-toolkit/Containerfile)
- [toolkit/rdma-validation-toolkit/bin/rdma-validate](./toolkit/rdma-validation-toolkit/bin/rdma-validate)
- [toolkit/rdma-validation-toolkit/README.ru.md](./toolkit/rdma-validation-toolkit/README.ru.md)

Что даёт этот набор:

- один образ для `TCP`, обычного `RDMA` и `GPUDirect RDMA`;
- одна утилита `rdma-validate`;
- подробные журналы в `/tmp/rdma-validation/*.log`;
- краткие структурированные результаты в `/tmp/rdma-validation/results.jsonl`;
- `report` для общей сводки;
- `check` для проверки по порогам.

Сборка образа:

```bash
cd toolkit/rdma-validation-toolkit
docker build -t registry.example/rdma-validation-toolkit:0.2.0 -f Containerfile .
docker push registry.example/rdma-validation-toolkit:0.2.0
```

Минимальная проверка по порогам:

```bash
export MIN_TCP_BW_GBPS=10
export MIN_RDMA_BW_GBPS=80
export MAX_TCP_LAT_USEC=80
export MAX_RDMA_LAT_USEC=10
export MIN_RDMA_OVER_TCP_RATIO=3
export MIN_GPUDIRECT_BW_GBPS=50
export MIN_GPUDIRECT_OVER_TCP_RATIO=3

rdma-validate check
```

Если хотя бы один порог не выполняется, `check` завершится с ошибкой.

## Типовые проблемы

### `modprobe nvidia-peermem` падает

Для `V100` это критическая проблема.

Порядок действий:

1. убедиться, что OFED уже установлен;
2. NVIDIA driver переустанавливается после OFED;
3. повторно проверить:

```bash
modprobe nvidia-peermem
lsmod | grep nvidia_peermem
```

Если перед этим OFED уже стоял, а модуль всё равно не поднимается, почти
всегда проблема в порядке установки: OFED был поставлен позже, чем NVIDIA
driver. На таком узле проще всего ещё раз запустить installer NVIDIA driver
поверх уже готового OFED.

### В `ib_write_bw --help` нет `--use_cuda`

Текущий `perftest` собран без поддержки CUDA. Переустанови версию с поддержкой
CUDA.

### `--use_cuda_dmabuf` падает на `V100`

Для этой базовой конфигурации `DMA-BUF` не считается рабочим путём. Используй
`nvidia-peermem` и `--use_cuda` / `--use_cuda_bus_id`.

### Pod получил сетевой интерфейс, но не видит verbs

Для Mellanox в текущем `sdn` проверяй именно `bindingMode: DPDK`.

Минимальная ручная проверка внутри pod:

```bash
ip -br link
ls -l /dev/infiniband
ibv_devices
ibv_devinfo -d mlx5_0 -v
```

Если интерфейс есть, а `/dev/infiniband/uverbs0` нет, проблема уже не в
адресации, а в довозе RDMA device внутрь pod.

### Тест не проходит, хотя адреса назначены правильно

В первую очередь проверяются четыре вещи:

1. нужный ли это прямой интерфейс, а не management-сеть;
2. совпадает ли `GID index` с текущим IP на интерфейсе;
3. использован ли правильный `mlx5_*` device;
4. совпадает ли выбранная GPU с той, которая ближе к NIC по `nvidia-smi topo -m`.

На проверенном стенде самая частая ошибка при ручном повторении была именно в
том, что после смены `/30` использовался старый `GID index`.

### Подготовка pod зависает на этапе prepare

На VM проверь:

- `vIOMMU`;
- `intel_iommu=on iommu=pt`;
- наличие `iommu_group`.

### После изменения underlay тесты ведут себя нестабильно

Пересоздай тестовый pod и связанные claims, чтобы исключить старое
распределение устройств.

## Что входит в документ

Документ покрывает:

- `RDMA`;
- `GPUDirect RDMA`;
- проверки на узле;
- проверки в pod;
- воспроизводимое сравнение по скорости и задержке.

В документ не входят:

- `NCCL` и распределённое обучение;
- политику планирования `GPU + NIC` с учётом топологии;
- автоматическую публикацию готового образа с инструментами;
- симметричный `~95 Gb/sec` как гарантированное свойство любой VM-конфигурации.
