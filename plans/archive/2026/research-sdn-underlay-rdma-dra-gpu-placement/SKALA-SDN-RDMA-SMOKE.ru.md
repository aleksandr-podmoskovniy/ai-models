# Проверка GPUDirect RDMA на RED OS 8

## Цель проверки

Этот документ отвечает на один практический вопрос: работает ли на стенде
именно `GPUDirect RDMA`, а не только обычный `RDMA`. Под рабочим
`GPUDirect RDMA` здесь понимается путь:

```text
GPU memory -> RDMA NIC -> сеть -> RDMA NIC -> GPU memory
```

Ниже собран путь от подготовки узла до проверки между подами. Обычный `RDMA`
и `TCP` используются только как контрольные замеры:

- если не работает обычный `RDMA`, сначала нужно чинить сеть, verbs и прямую
  сеть;
- если обычный `RDMA` работает, а `--use_cuda` нет, проблема уже в пути
  `GPU memory -> RDMA`;
- если `GPUDirect RDMA` работает, но скорость ниже ожидаемой, тогда уже имеет
  смысл разбирать связку GPU и сетевого адаптера, `GID index`, параметры теста и
  асимметрию конкретной виртуализованной среды.

Проверку можно считать успешной, когда одновременно выполнены следующие
условия:

- на узле поднимается `nvidia-peermem`;
- `perftest` на узле поддерживает `--use_cuda`;
- межузловой `ib_write_bw --use_cuda=0` проходит без ошибок;
- под получает GPU;
- под получает прямой сетевой интерфейс;
- в поде есть `/dev/infiniband/uverbs*`;
- тест `ib_write_bw --use_cuda=0` между подами проходит без ошибок;
- лучший скоростной профиль `ib_read_bw --use_cuda_bus_id ... -q 8` на этом
  стенде выходит на уровень около `95 Gb/sec`.

## Стенд и что на нём настроено

Сначала зафиксирован сам стенд: какие узлы участвовали в проверке, какие
рабочие связки `GPU + RDMA` на нём были и какой программный baseline в итоге
получился.

### Что есть на стенде

Проверка выполнялась на двух узлах:

| Узел | Имя узла                  | Тип             | Прямой IP для проверки на узле | Что важно |
| ---- | ------------------------- | --------------- | ------------------------------ | --------- |
| `w1` | `k8s-dvp-w1-gpu.apiac.ru` | физический узел | `192.168.3.60`                 | на узле используются две рабочие связки `GPU + RDMA` |
| `w3` | `k8s-dvp-w3-gpu.apiac.ru` | VM на `PVE`     | `192.168.3.32`                 | на узле используется одна рабочая связка `GPU + RDMA` |

На стенде были доступны три рабочие связки `GPU + RDMA`:

| Узел | Под                   | GPU                | Прямой интерфейс | Прямой IP         | Назначение |
| ---- | --------------------- | ------------------ | ---------------- | ----------------- | ---------- |
| `w1` | `gpudirect-pod-w1-c2` | `00000000:C2:00.0` | `enp193s0np0`    | `172.31.130.1/30` | основной под на `w1` для тестов между узлами и между подами |
| `w3` | `gpudirect-pod-w3-01` | `00000000:01:00.0` | `enp2s0np0`      | `172.31.130.2/30` | основной под на `w3` для тестов между узлами и между подами |
| `w1` | `gpudirect-pod-w1-80` | `00000000:81:00.0` | `enp130s0np0`    | не назначался     | запасной локальный под на второй рабочей паре `GPU + RDMA` |

Для чтения документа достаточно держать в голове три вещи:

- основная пара для всех тестов между узлами и между подами:
  `gpudirect-pod-w1-c2` на `w1` и `gpudirect-pod-w3-01` на `w3`;
- запасная локальная связка на `w1` нужна, чтобы быстро проверить `GPU`,
  verbs и доступность прямого интерфейса, если не хочется сразу идти в
  межузловую проверку;
- для основной пары после назначения адресов используются
  `172.31.130.1/30 <-> 172.31.130.2/30` и `GID index 3`.

Иными словами, при двух узлах здесь не случайно фигурируют три пода: на `w1`
есть две рабочие пары `GPU + RDMA`, поэтому на нём поднимаются два тестовых
пода. Основной межузловой сценарий идёт через `gpudirect-pod-w1-c2` и
`gpudirect-pod-w3-01`. Под `gpudirect-pod-w1-80` нужен только как запасная
локальная точка проверки на `w1`, поэтому отдельный прямой IP для него в этом
документе не используется.

Для этого стенда важна ещё одна практическая деталь: `w3` живёт как VM на
`PVE`. Для таких узлов требования к виртуализации надо закрывать сразу, ещё до
подготовки `RDMA` и запуска подов:

- включить `vIOMMU` в гостевой системе;
- добавить в kernel cmdline гостевой системы `intel_iommu=on iommu=pt`;
- убедиться, что у проброшенного Mellanox PF виден `iommu_group`.

Именно отсутствие `vIOMMU` и `iommu_group` на `w3` ломало подготовку
устройств внутри `sdn`.

Ниже по документу слова `сервер` и `клиент` означают только роли конкретного
запуска `perftest`, а не постоянные роли узлов или подов на стенде.

### Что в итоге установлено на узле

Проверенная конфигурация:

- `RED OS 8.0.2`
- kernel `6.12.37-1.red80.x86_64`
- Mellanox 100GbE с `RoCE v2`
- NVIDIA `Tesla V100-PCIE-32GB`
- `MLNX_OFED_SRC-24.10-3.2.5.0`
- `perftest` с поддержкой CUDA

На подготовленном узле в итоге были такие версии:

```text
kernel-lt            6.12.37-1.red80
kernel-lt-devel      6.12.37-1.red80
ofed-scripts         24.10-OFED.24.10.3.2.5
rdma-core            2410mlnx54-1.2410068
ucx                  1.18.0-1.2410068
openmpi              4.1.7rc1-1.2410325
perftest             24.10.0-0.95.g370212b.2410325
mpitests_openmpi     3.2.24-2ffc2d6.2410068
```

Для `V100` в этом документе рабочим считается путь через `nvidia-peermem`, а
не через `DMA-BUF`. Поэтому здесь важны три вещи:

- `modprobe nvidia-peermem` должен проходить без ошибки;
- `perftest` должен показывать `--use_cuda` и `--use_cuda_bus_id`;
- реальным доказательством считается только успешный запуск с `--use_cuda`.

### Откуда брать пакеты и файлы

Все проверенные артефакты для такого же узла собраны здесь:
[архив с пакетами и вспомогательными файлами](https://disk.360.yandex.ru/d/7LWtFzDegnTQjA).

| Что нужно | Где лежит |
| --------- | --------- |
| Архив OFED | `/root/MLNX_OFED_SRC-24.10-3.2.5.0.tgz` |
| Распакованный каталог OFED | `/root/MLNX_OFED_SRC-24.10-3.2.5.0` |
| RPM-пакеты OFED | `/root/MLNX_OFED_SRC-24.10-3.2.5.0/RPMS/redos-release-8.0.2-37.red80/x86_64/*.rpm` |
| Пересобранный `perftest` с CUDA | `/root/rpmbuild-perftest-cuda/RPMS/x86_64/perftest-24.10.0-0.95.g370212b.2410325.x86_64.rpm` |
| Пересобранный `mpitests_openmpi` | `/root/rpmbuild-mpitests-fixed/RPMS/x86_64/mpitests_openmpi-3.2.24-2ffc2d6.2410068.x86_64.rpm` |
| Заголовок CUDA для пересборки `perftest` | `/root/w1-cuda.h` |
| Репозиторий `nvidia-container-toolkit` | `https://nvidia.github.io/libnvidia-container/stable/rpm/nvidia-container-toolkit.repo` |

## Маршрут проверки

Документ лучше проходить именно в таком порядке:

1. Если узел ещё не подготовлен, сначала выполнить шаг `1`.
2. На каждом узле закрыть обязательные предпосылки `GPUDirect RDMA`.
3. На узлах доказать, что работает путь `GPU memory -> RDMA`.
4. Проверить, что под реально получает всё необходимое для `GPUDirect RDMA`.
5. На основной паре подов повторить функциональный тест, тест задержки и
   лучший скоростной профиль.
6. Только после этого, при необходимости, снимать контрольные замеры обычного
   `RDMA` и `TCP`.

Если на любом шаге выпадает обязательное условие, дальше идти не нужно.
Сначала нужно исправить ближайший сломанный слой, а потом повторять проверку.

## Ориентиры по результатам

Это не SLA и не универсальная гарантия для любого железа. Ниже приведены ориентиры,
полученные именно на проверенном стенде.

### Основные ориентиры по `GPUDirect RDMA`

| Сценарий                                        | Команда                                 | Ожидаемый уровень        |
| ----------------------------------------------- | --------------------------------------- | ------------------------ |
| Узел: функциональное подтверждение               | `ib_write_bw --use_cuda=...`            | `~56-57 Gb/sec` или выше |
| Узел: лучший скоростной профиль                  | `ib_read_bw --use_cuda_bus_id=... -q 8` | около `95 Gb/sec`        |
| Под: функциональное подтверждение               | `ib_write_bw --use_cuda=...`            | `~56.46 Gb/sec`          |
| Под: задержка из GPU-памяти                     | `ib_send_lat --use_cuda=... -s 64`      | `~4.57 usec`             |
| Под: лучший скоростной профиль                  | `ib_read_bw --use_cuda_bus_id=... -q 8` | `~95.52 Gb/sec`          |

### Контрольные ориентиры по обычному `RDMA` и `TCP`

| Сценарий                                                  | Команда                          | Ожидаемый уровень   |
| --------------------------------------------------------- | -------------------------------- | ------------------- |
| Узел: обычный `RDMA`                                      | `ib_write_bw`                    | около `86 Gb/sec`   |
| Под: обычный `RDMA`, пропускная способность              | `ib_read_bw -q 8`                | `~97.30 Gb/sec`     |
| Под: обычный `RDMA`, задержка                            | `ib_send_lat -s 64`              | `~2.95 usec`        |
| Под: `TCP` без отдельной настройки, пропускная способность | `iperf3 -P 4 -t 10`              | `~15.8-23.6 Gb/sec` |
| Под: `TCP` без отдельной настройки, задержка             | `sockperf ping-pong --tcp -m 64` | `~37.48 usec`       |

Как эти цифры читать:

- для `V100` важнее сначала получить устойчивый успешный `--use_cuda` тест,
  чем сразу смотреть на лучшую скорость;
- функциональное подтверждение и лучший скоростной профиль — это два разных
  теста и две разные цели;
- обычный `RDMA` и `TCP` нужны как контроль: они помогают понять, проблема в
  сети вообще, на уровне verbs или уже в пути через GPU-память.

## Шаг 1. Подготовка узла

На уже подготовленном узле этот раздел можно пропустить.

### 1.1. Целевое ядро и параметры `IOMMU`

Проверенная комбинация:

- `RED OS 8.0.2`
- kernel `6.12.37-1.red80.x86_64`

Установка ядра:

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

Проверка:

```bash
uname -r
cat /proc/cmdline
```

Исправное состояние:

- `uname -r` возвращает `6.12.37-1.red80.x86_64`;
- в `cmdline` видны `intel_iommu=on iommu=pt`.

### 1.2. Требования к хосту и виртуализации

На физическом узле должны выполняться три базовых условия:

- Mellanox NIC видна в PCI;
- GPU видна в PCI;
- `IOMMU` не ломает `1:1 passthrough`.

Для VM дополнительно нужны:

- `vIOMMU` в гостевой системе;
- `intel_iommu=on iommu=pt` в kernel cmdline гостевой системы;
- видимый `iommu_group` у проброшенного Mellanox PF.

На этом стенде это относится к `w3`, который работает как VM на `PVE`.

Быстрая проверка:

```bash
readlink -f /sys/bus/pci/devices/0000:02:00.0/iommu_group
```

Если `iommu_group` не виден, а устройство пробрасывается как PCI device,
дальнейшая подготовка пода может завершаться с ошибкой.

### 1.3. Пакеты для сборки

Минимальный набор пакетов для `RED OS 8`:

```bash
dnf install -y \
  gcc gcc-c++ make rpm-build \
  autoconf automake libtool cmake \
  libnl3-devel python3-devel systemd-devel \
  python3-Cython python3-docutils pandoc \
  pciutils-devel
```

### 1.4. Где лежат пакеты и вспомогательные файлы

Для шагов ниже используются те же артефакты, которые уже перечислены выше в
разделе `Откуда брать пакеты и файлы`.

### 1.5. Установка Mellanox OFED

Исходники на проверенной конфигурации:

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

После установки узел перезагружается, затем проверяются базовые пакеты:

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

### 1.6. Переустановка NVIDIA driver после OFED

Для `V100` это обязательный шаг.

Если NVIDIA driver был поставлен до OFED, `nvidia-peermem` может оказаться
собранным против старого состояния RDMA symbols и не загрузиться.

После установки OFED порядок такой:

1. повторно запускается installer NVIDIA driver;
2. проверяется, что пересобрался `nvidia-peermem.ko`;
3. включается автозагрузка модуля.

Команды:

```bash
sh ./NVIDIA-Linux-x86_64-580.76.05.run \
  --skip-module-load \
  --no-dkms \
  --kernel-module-type=proprietary

printf 'nvidia-peermem\n' >/etc/modules-load.d/nvidia-peermem.conf
modprobe nvidia-peermem
```

Проверка:

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

### 1.7. Проверка `perftest` с поддержкой CUDA

Если штатный `perftest` уже показывает `--use_cuda`, достаточно проверки.
Если нет, пакет заменяется на пересобранный RPM с поддержкой CUDA.

На проверенном стенде для `w3` использовался такой пакет:

```bash
rpm -Uvh --replacepkgs --nodeps \
  /root/rpmbuild-perftest-cuda/RPMS/x86_64/perftest-24.10.0-0.95.g370212b.2410325.x86_64.rpm
```

Проверка:

```bash
ib_write_bw --help 2>&1 | egrep -i 'use_cuda|cuda_bus_id|cuda_dmabuf'
```

Минимально ожидаются:

- `--use_cuda`
- `--use_cuda_bus_id`

Пример исправного вывода:

```text
  --use_cuda=<gpu index>      use CUDA memory
  --use_cuda_bus_id=<pci id>  use CUDA device by PCI bus ID
  --use_cuda_dmabuf           use CUDA DMA-BUF memory
```

### 1.8. Установка `nvidia-container-toolkit`

Если на узле потом будут запускаться поды с GPU, этот набор инструментов
удобно поставить сразу:

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

### 1.9. Необязательный пакет `mpitests_openmpi`

Если кроме `perftest` нужен комплект MPI-бенчмарков:

```bash
rpm -Uvh --replacepkgs \
  /root/rpmbuild-mpitests-fixed/RPMS/x86_64/mpitests_openmpi-3.2.24-2ffc2d6.2410068.x86_64.rpm

rpm -q mpitests_openmpi
```

## Шаг 2. Закрыть обязательные предпосылки на узле

К шагу `3` имеет смысл переходить только после того, как одновременно
закрыты:

- обычный `RDMA` на узле;
- `nvidia-peermem`;
- `perftest` с CUDA-флагами.

### 2.1. Проверка обычного `RDMA` на узле

```bash
rdma link show
ibv_devices
ibv_devinfo -d mlx5_0 -v | egrep 'hca_id|state:|link_layer|active_mtu'
```

Исправное состояние:

- есть `mlx5_0` или `mlx5_1`;
- `PORT_ACTIVE`;
- `link_layer: Ethernet`;
- `active_mtu: 4096`.

### 2.2. Проверка предпосылок для `GPUDirect RDMA`

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

Если любой из этих пунктов не выполнен, дальше идти рано.

## Шаг 3. Доказать `GPUDirect RDMA` на узле

Сначала доказательство должно пройти на самих узлах, без Kubernetes.

Если нужна только host-side проверка того, что `GPUDirect RDMA` вообще жив,
достаточно шага `3.2`. Подпункт `3.3` нужен уже для выхода на лучший
скоростной профиль.

### 3.1. Определить прямой IP, `GID index` и пару GPU с сетевым адаптером

```bash
ip -br addr
ibv_devinfo -d mlx5_0 -v | sed -n '/GID\\[/,/^$/p'
nvidia-smi --query-gpu=index,pci.bus_id,name --format=csv
nvidia-smi topo -m
```

При выборе параметров важны четыре вещи:

- использовать прямой IP, а не адрес из управляющей или overlay-сети;
- выбрать правильный `RoCE v2 GID index` для этого IP;
- выбрать нужный `mlx5_*`;
- выбрать GPU, которая по `PCIe` ближе к выбранному сетевому адаптеру.

На проверенном стенде:

- `w1`: IP `192.168.3.60`, `mlx5_0`, `GID index 1`, `GPU1`,
  `00000000:C2:00.0`;
- `w3`: IP `192.168.3.32`, `mlx5_0`, `GID index 1`, `GPU0`,
  `00000000:01:00.0`.

### 3.2. Функциональное подтверждение: `ib_write_bw --use_cuda=0`

Этот тест нужен как доказательство самого пути `GPU memory -> RDMA`.

Сервер:

```bash
ib_write_bw -d mlx5_0 -x <gid_index> -m 4096 -F --report_gbits -D 5 --use_cuda=0
```

Клиент:

```bash
ib_write_bw -d mlx5_0 -x <gid_index> -m 4096 -F --report_gbits -D 5 --use_cuda=0 <peer_ip>
```

Исправный результат:

- команда завершается с `RC=0`;
- в выводе есть `initializing CUDA`;
- в выводе есть `cuMemAlloc()`.

Рабочий пример со стенда.

Сервер на `w1`:

```bash
ib_write_bw -d mlx5_0 -x 1 -m 4096 -F --report_gbits -D 5 --use_cuda=0
```

Клиент на `w3`:

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

### 3.3. Лучший скоростной профиль: `ib_read_bw --use_cuda_bus_id ... -q 8`

После функционального подтверждения имеет смысл снять лучший скоростной
профиль. На этом стенде он получался не через `ib_write_bw`, а через
`ib_read_bw` с точной
привязкой к GPU по `PCI BUS ID`.

Если `ib_read_bw` с CUDA уже лежит в `PATH`, его можно запускать напрямую. На
проверенном `w1` в примере ниже использовалась локальная сборка из каталога
`/root/perftest-cuda-build-w1/perftest-24.10.0/`.

Сервер:

```bash
ib_read_bw \
  -d mlx5_0 -x <gid_index> -q 8 -m 4096 -F --report_gbits -D 10 \
  --use_cuda_bus_id=<server_gpu_bus_id>
```

Клиент:

```bash
ib_read_bw \
  -d mlx5_0 -x <gid_index> -q 8 -m 4096 -F --report_gbits -D 10 \
  --use_cuda_bus_id=<client_gpu_bus_id> \
  <peer_ip>
```

Рабочий пример со стенда.

Сервер на `w1`:

```bash
/root/perftest-cuda-build-w1/perftest-24.10.0/ib_read_bw \
  -d mlx5_0 -x 1 -q 8 -m 4096 -F --report_gbits -D 10 \
  --use_cuda_bus_id=00000000:C2:00.0
```

Клиент на `w3`:

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

На этом же стенде была заметная асимметрия: если поменять роли местами и
сделать `w3` удалённым источником GPU-памяти, скорость оставалась около
`61.7 Gb/sec`. Это важно учитывать при интерпретации результатов в
виртуализованной среде.

## Шаг 4. Что под должен получить для `GPUDirect RDMA`

К моменту начала проверки между подами под должен получить не только обычный
сетевой интерфейс, но и весь набор, без которого тест `--use_cuda` не имеет
смысла.

### 4.1. Прямая сеть для Mellanox

Для Mellanox в `sdn` используется:

```json
"bindingMode": "DPDK"
```

Практический смысл этого выбора:

- устройство остаётся на `mlx5_core`;
- под получает обычный сетевой интерфейс;
- под получает verbs-устройства;
- внутри пода работают `ibv_devinfo`, `ib_write_bw`, `ib_send_lat`.

Для обычной проверки verbs и `GPUDirect RDMA` здесь не нужен `VFIO-PCI`.

### 4.2. Минимальные требования к безопасности пода

Подтверждённый минимальный набор:

- без `hostPath`;
- без `privileged: true`;
- `IPC_LOCK` нужен для verbs-тестов;
- `NET_ADMIN` нужен только если прямой IP назначается вручную;
- `NET_RAW` нужен только если дополнительно используется `ping`.

Если среда сама выдаёт IP на прямой интерфейс, тестовый под можно сократить до
одного `IPC_LOCK`.

### 4.3. Базовый шаблон пода

Ниже — минимальный шаблон пода с инструментами для прямой сети и `RDMA`. Для
проверки `GPUDirect RDMA` этот под дополнительно должен получать GPU тем
способом, который уже принят в конкретном кластере.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: gpudirect-tools
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
  # Запрос GPU добавляется здесь тем механизмом,
  # который уже используется в этом кластере.
```

Если нужен ручной тест с временным `/30` и `ping`, дополнительные capabilities
добавляются точечно:

- добавить `NET_ADMIN`, если IP назначается руками;
- добавить `NET_RAW`, если нужен `ping`.

### 4.4. Что проверить внутри пода до запуска `--use_cuda`

До теста между подами внутри пода должны одновременно выполняться следующие
условия:

- видна GPU через `nvidia-smi`;
- есть прямой сетевой интерфейс;
- есть `/dev/infiniband/uverbs*`;
- `ib_write_bw --help` показывает `--use_cuda`.

Если хотя бы один из этих пунктов не выполнен, к шагу `6` переходить рано.

## Шаг 5. Подготовить основную пару подов

Ниже используется уже поднятая рабочая пара:

- `gpudirect-pod-w1-c2` на `k8s-dvp-w1-gpu.apiac.ru`;
- `gpudirect-pod-w3-01` на `k8s-dvp-w3-gpu.apiac.ru`.

### 5.1. Проверка интерфейса и verbs devices

Вручную:

```bash
ip -br link
ls -l /dev/infiniband
ibv_devices
ibv_devinfo -d mlx5_0 -v | egrep 'hca_id|state:|link_layer|active_mtu'
```

Что должно быть:

- внутри пода есть прямой сетевой интерфейс;
- есть `/dev/infiniband/uverbs0`;
- `ibv_devinfo` показывает `PORT_ACTIVE`, `link_layer=Ethernet`,
  `active_mtu=4096`.

На проверенном стенде:

- `gpudirect-pod-w1-c2` использует `enp193s0np0`;
- `gpudirect-pod-w3-01` использует `enp2s0np0`.

### 5.2. Назначение прямых IP-адресов

Этот шаг нужен только если среда сама не выдала адрес на прямой интерфейс.

Команды со стенда:

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

Проверка:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w1-c2 -- ip -4 -br addr
kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- ip -4 -br addr
```

То же самое через `rdma-validate`.

Под на стороне сервера:

```bash
export DIRECT_IFACE=<iface>
export DIRECT_IP=172.31.130.1/30
rdma-validate bind-ip
```

Под на стороне клиента:

```bash
export DIRECT_IFACE=<iface>
export DIRECT_IP=172.31.130.2/30
rdma-validate bind-ip
```

### 5.3. Проверка `GID index`

`GID index` нельзя брать из памяти. Его надо смотреть после назначения IP.

```bash
ibv_devinfo -d mlx5_0 -v | sed -n '/GID\\[/,/^$/p'
```

На проверенном стенде после назначения `/30` использовался `GID index 3`.

### 5.4. Проверка GPU и `perftest` с поддержкой CUDA в поде

```bash
nvidia-smi -L
ls -l /dev/infiniband
ib_write_bw --help | egrep -i 'use_cuda|cuda_bus_id|cuda_dmabuf'
```

Исправное состояние:

- GPU видна в `nvidia-smi`;
- verbs-устройства доступны;
- `ib_write_bw --help` показывает CUDA flags.

Если в поде штатного `perftest` с поддержкой CUDA нет, его можно собрать прямо
внутри пода.

Рабочая последовательность со стенда:

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

Для шагов `5-7` используются те же поды, GPU, интерфейсы и адреса, которые уже
сведены выше в разделе `Что есть на стенде`. Для `gpudirect-pod-w1-80` в этом
runbook прямой IP не приводится, потому что он нужен только для локальной
проверки на `w1`, а не для межузлового теста.

Быстрая локальная проверка третьего пода:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w1-80 -- \
  bash -lc 'nvidia-smi -L; ls -l /dev/infiniband; ip -br link; ib_write_bw --help | egrep -i "use_cuda|cuda_bus_id|cuda_dmabuf"'
```

## Шаг 6. Доказать `GPUDirect RDMA` между подами

Только после шагов `4` и `5` есть смысл запускать собственно проверку между
подами.

Если задача только доказать факт рабочего `GPUDirect RDMA`, достаточно шага
`6.1`. Подпункты `6.2` и `6.3` нужны уже для задержки и максимального
скоростного профиля.

### 6.1. Функциональное подтверждение: `ib_write_bw --use_cuda=0`

Это основной тест. Его задача — доказать, что путь `GPU memory -> RDMA`
реально работает.

Сервер на `w1`:

```bash
kubectl -n sdn-rdma-test exec -it gpudirect-pod-w1-c2 -- \
  ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 5 --use_cuda=0
```

Клиент на `w3`:

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

Клиентский вывод со стенда:

```text
initializing CUDA
device name = [Tesla V100-PCIE-32GB]
cuMemAlloc() of a 131072 bytes GPU buffer
...
BW average[Gb/sec] 56.46
```

Более полный фрагмент:

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

Для `V100` этого теста достаточно, чтобы считать `GPUDirect RDMA`
функционально доказанным.

### 6.2. Задержка из памяти GPU: `ib_send_lat --use_cuda=0`

Этот тест показывает задержку на том же канале и тоже идёт из памяти GPU.

Сервер на `w1`:

```bash
kubectl -n sdn-rdma-test exec -it gpudirect-pod-w1-c2 -- \
  ib_send_lat -d mlx5_0 -x 3 -s 64 -n 2000 -F --use_cuda=0
```

Клиент на `w3`:

```bash
kubectl -n sdn-rdma-test exec -it gpudirect-pod-w3-01 -- \
  ib_send_lat -d mlx5_0 -x 3 -s 64 -n 2000 -F --use_cuda=0 172.31.130.1
```

Результат со стенда:

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

Как и в bandwidth-тесте, в выводе должны присутствовать `initializing CUDA`
и `cuMemAlloc()`.

### 6.3. Лучший скоростной профиль: `ib_read_bw --use_cuda_bus_id ... -q 8`

После успешного функционального подтверждения можно снимать лучший скоростной
профиль на той же паре подов.

Сервер на `w1`:

```bash
kubectl -n sdn-rdma-test exec -it gpudirect-pod-w1-c2 -- \
  ib_read_bw -d mlx5_0 -x 3 -q 8 -m 4096 -F --report_gbits -D 5 \
  --use_cuda_bus_id=00000000:C2:00.0
```

Клиент на `w3`:

```bash
kubectl -n sdn-rdma-test exec -it gpudirect-pod-w3-01 -- \
  ib_read_bw -d mlx5_0 -x 3 -q 8 -m 4096 -F --report_gbits -D 5 \
  --use_cuda_bus_id=00000000:01:00.0 172.31.130.1
```

Клиентский вывод со стенда:

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

И здесь обязательны признаки живого пути через GPU-память:

- `initializing CUDA`;
- `Picking GPU number 0` или аналогичное сообщение выбора устройства;
- `cuMemAlloc()`.

### 6.4. Короткий итог по основной паре подов

| Проверка                                      | Под на `w1`           | Под на `w3`           | Результат         |
| --------------------------------------------- | --------------------- | --------------------- | ----------------- |
| `GPUDirect RDMA`: функциональное подтверждение | `gpudirect-pod-w1-c2` | `gpudirect-pod-w3-01` | `56.46 Gb/sec`    |
| `GPUDirect RDMA`: задержка                    | `gpudirect-pod-w1-c2` | `gpudirect-pod-w3-01` | `t_avg 4.57 usec` |
| `GPUDirect RDMA`: лучший скоростной профиль   | `gpudirect-pod-w1-c2` | `gpudirect-pod-w3-01` | `95.52 Gb/sec`    |

## Шаг 7. Контрольные замеры обычного `RDMA` и `TCP`

Этот раздел не заменяет шаг `6`. Он нужен только после того, как
`GPUDirect RDMA` уже доказан, и требуется сравнить тот же прямой канал с
обычным `RDMA` и `TCP`.

Для всех замеров ниже используется основная пара из сводки выше:
`gpudirect-pod-w1-c2` (`enp193s0np0`, `172.31.130.1/30`) и
`gpudirect-pod-w3-01` (`enp2s0np0`, `172.31.130.2/30`).

### 7.1. Обычный `RDMA`: пропускная способность

Сервер:

```bash
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w1-c2 -- \
  bash -lc 'ib_read_bw -d mlx5_0 -x 3 -q 8 -m 4096 -F --report_gbits -D 5'
```

Клиент:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  bash -lc 'ib_read_bw -d mlx5_0 -x 3 -q 8 -m 4096 -F --report_gbits -D 5 172.31.130.1'
```

Результат со стенда:

```text
#bytes     #iterations    BW peak[Gb/sec]    BW average[Gb/sec]
65536      556761         0.00               97.30
```

### 7.2. Обычный `RDMA`: задержка

Сервер:

```bash
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w1-c2 -- \
  bash -lc 'ib_send_lat -d mlx5_0 -x 3 -s 64 -n 2000 -F'
```

Клиент:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  bash -lc 'ib_send_lat -d mlx5_0 -x 3 -s 64 -n 2000 -F 172.31.130.1'
```

Результат со стенда:

```text
 t_min[usec]  t_max[usec]  t_typical[usec]  t_avg[usec]  t_stdev[usec]
 2.86         64.89        2.90             2.95         0.41
```

### 7.3. `TCP`: пропускная способность

Если `iperf3` и `sockperf` ещё не стоят в подах, их надо доустановить:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w1-c2 -- \
  bash -lc 'export DEBIAN_FRONTEND=noninteractive; apt-get update && apt-get install -y iperf3 sockperf'

kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  bash -lc 'export DEBIAN_FRONTEND=noninteractive; apt-get update && apt-get install -y iperf3 sockperf'
```

Сервер на `w1`:

```bash
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w1-c2 -- \
  bash -lc 'iperf3 -s -B 172.31.130.1 -1'
```

Клиент на `w3`:

```bash
kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  bash -lc 'iperf3 -c 172.31.130.1 -B 172.31.130.2 -P 4 -t 10'
```

В этом направлении на стенде получилось:

```text
[SUM]   0.00-10.00  sec  18.4 GBytes  15.8 Gbits/sec
```

Проверка в обратном направлении:

```bash
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w3-01 -- \
  bash -lc 'iperf3 -s -B 172.31.130.2 -1'

kubectl -n sdn-rdma-test exec gpudirect-pod-w1-c2 -- \
  bash -lc 'iperf3 -c 172.31.130.2 -B 172.31.130.1 -P 4 -t 10'
```

Во втором направлении получилось:

```text
[SUM]   0.00-10.00  sec  27.4 GBytes  23.6 Gbits/sec
```

### 7.4. `TCP`: задержка

Сервер на `w1`:

```bash
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w1-c2 -- \
  bash -lc 'sockperf server --tcp -i 172.31.130.1'
```

Клиент на `w3`:

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

Итоговый средний RTT:

```text
avg-rtt=79.663 usec
```

### 7.5. Сводка по контрольным замерам

| Метрика                                         | Результат          |
| ----------------------------------------------- | ------------------ |
| `GPUDirect RDMA`: функциональное подтверждение  | `56.46 Gb/sec`     |
| `GPUDirect RDMA`: задержка                     | `4.57 usec`        |
| `GPUDirect RDMA`: лучший скоростной профиль     | `95.52 Gb/sec`     |
| Обычный `RDMA`: пропускная способность          | `97.30 Gb/sec`     |
| Обычный `RDMA`: задержка                        | `2.95 usec`        |
| `TCP`: пропускная способность                   | `15.8-23.6 Gb/sec` |
| `TCP`: задержка                                 | `37.48 usec`       |
| `TCP`: RTT                                      | `79.663 usec`      |

Практический вывод:

- если обычный `RDMA` уже даёт около `97 Gb/sec`, а `GPUDirect RDMA` на
  `ib_write_bw --use_cuda=0` ниже, это не означает автоматически, что
  `GPUDirect RDMA` не работает;
- для `V100` `ib_write_bw --use_cuda=0` здесь служит именно функциональным
  подтверждением пути `GPU memory -> RDMA`;
- лучший скоростной профиль для `GPUDirect RDMA` на этом стенде достигается
  отдельным тестом `ib_read_bw --use_cuda_bus_id ... -q 8`;
- по сравнению с `TCP` обычный `RDMA` на том же канале быстрее примерно в
  `4.1-6.2x` по пропускной способности и примерно в `12.7x` по задержке.

## Шаг 8. Набор инструментов для повторной проверки

Если не хочется повторять все команды руками, рядом с этим документом в
репозитории лежит готовый набор файлов:

- [toolkit/rdma-validation-toolkit/Containerfile](./toolkit/rdma-validation-toolkit/Containerfile)
- [toolkit/rdma-validation-toolkit/bin/rdma-validate](./toolkit/rdma-validation-toolkit/bin/rdma-validate)
- [toolkit/rdma-validation-toolkit/README.ru.md](./toolkit/rdma-validation-toolkit/README.ru.md)

Что даёт этот набор:

- один образ для `TCP`, обычного `RDMA` и `GPUDirect RDMA`;
- одну утилиту `rdma-validate`;
- подробные журналы в `/tmp/rdma-validation/*.log`;
- краткие структурированные результаты в
  `/tmp/rdma-validation/results.jsonl`;
- команду `report` для сводки;
- команду `check` для проверки по порогам.

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

## Шаг 9. Типовые проблемы

### `modprobe nvidia-peermem` падает

Для `V100` это критическая проблема.

Порядок действий:

1. убедиться, что OFED уже установлен;
2. переустановить NVIDIA driver после OFED;
3. повторно проверить:

```bash
modprobe nvidia-peermem
lsmod | grep nvidia_peermem
```

Если OFED уже стоял, а модуль всё равно не поднимается, чаще всего проблема в
порядке установки: OFED был поставлен позже, чем NVIDIA driver.

### В `ib_write_bw --help` нет `--use_cuda`

Текущий `perftest` собран без поддержки CUDA. Надо переустановить или
пересобрать версию с поддержкой CUDA.

### `--use_cuda_dmabuf` падает на `V100`

Для этой конфигурации `DMA-BUF` не считается рабочим путём. Используется
`nvidia-peermem` и флаги `--use_cuda` / `--use_cuda_bus_id`.

### Под получил интерфейс, но не видит RDMA-устройства

Для Mellanox в `sdn` надо проверять именно `bindingMode: DPDK`.

Минимальная проверка внутри пода:

```bash
ip -br link
ls -l /dev/infiniband
ibv_devices
ibv_devinfo -d mlx5_0 -v
```

Если интерфейс есть, а `/dev/infiniband/uverbs0` нет, проблема не в адресации,
а в том, что RDMA-устройство не попало внутрь пода.

### Обычный `RDMA` проходит, а `--use_cuda` нет

В таком случае проблема уже не в самой сети. В первую очередь надо проверить:

- `modprobe nvidia-peermem`;
- наличие CUDA-флагов в `perftest`;
- правильность выбора пары GPU и сетевого адаптера;
- актуальный `GID index`.

### Тест не проходит, хотя адреса назначены правильно

В первую очередь проверяются четыре вещи:

1. правильный ли это прямой интерфейс;
2. совпадает ли `GID index` с текущим IP на интерфейсе;
3. использован ли правильный `mlx5_*`;
4. совпадает ли выбранная GPU с той, которая ближе к сетевому адаптеру по
   `nvidia-smi topo -m`.

На стенде самая частая ошибка при ручном повторении была в том, что после
смены `/30` использовался старый `GID index`.

### Подготовка пода зависает на этапе `prepare`

Если это узел в VM, сначала надо проверять виртуализацию, а не Kubernetes:

- `vIOMMU`;
- `intel_iommu=on iommu=pt`;
- наличие `iommu_group`.

На этом стенде это относится к `w3` на `PVE`: без этих условий `sdn` зависал
на подготовке устройств.

### После изменения прямой сети тесты ведут себя нестабильно

Надо пересоздать тестовый под и связанные объекты выделения устройств, чтобы
исключить старое распределение устройств.
