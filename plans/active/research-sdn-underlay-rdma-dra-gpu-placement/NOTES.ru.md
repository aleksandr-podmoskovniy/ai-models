# Findings

Это инженерный log и длинная история расследования. Каноническая короткая
инструкция для коллег живёт в `SKALA-SDN-RDMA-SMOKE.ru.md`; здесь остаются
подробные промежуточные блокеры, rebuild paths, benchmark branches и raw
findings.

## 1. Как `sdn` сейчас пробрасывает устройство в pod

Текущий underlay flow в `sdn` выглядит так:

1. Пользователь указывает в pod annotation `network.deckhouse.io/networks-spec`
   объект `{"type":"UnderlayNetwork","name":"..."}`.
2. Admission webhook добавляет в pod `spec.resourceClaims` и
   `containers[].resources.claims` для соответствующего `ResourceClaimTemplate`.
3. `UnderlayNetwork` controller создаёт:
   - `DeviceClass` с CEL selector по `underlayNetwork`;
   - `ResourceClaimTemplate` с `deviceClassName=d8-sdn-<underlay>`.
4. Node-local DRA controller публикует `resource.k8s.io` devices из matched PF/VF
   в `ResourceSlice`.
5. DRA kubelet plugin в `PrepareResourceClaims`:
   - находит выбранные `NodeNetworkInterface`;
   - валидирует `bindingMode`;
   - переключает binding через `interface-syncer`;
   - формирует CDI spec;
   - либо сохраняет VFIO allocation, либо netdev handoff для CNI.
6. CNI-side reconciler:
   - для `NetDev`/Mellanox `DPDK` двигает netdev в pod netns;
   - для userspace/VFIO path пишет status annotation с VFIO metadata.

Ключевой вывод: scheduler extender `sdn` сейчас не участвует в выборе
`UnderlayNetwork`. Он фильтрует обычные `Network`-attachments и прямо исключает
`UnderlayNetwork` из логики. Для underlay selection проект уже опирается на
native DRA scheduling.

## 2. Что уже есть для RDMA

`sdn` нативно не имеет отдельного `bindingMode: RDMA`, но для Mellanox уже есть
важный baseline:

- `bindingMode: DPDK` для Mellanox не переводит устройство в `vfio-pci`, а
  оставляет `mlx5_core`;
- в pod попадает сам netdev;
- в CDI уже монтируются `uverbs` device files из `/dev/infiniband/*`;
- в pod монтируется PCI sysfs path устройства.

Это означает:

- для Mellanox текущий `DPDK` режим уже ближе к userspace RDMA/verbs path, чем к
  классическому VFIO-only DPDK path;
- попытка сделать RDMA через `vfio-pci` для Mellanox была бы неверным default:
  VFIO полезен для userspace PCI ownership, но не является естественным способом
  дать контейнеру verbs/RDMA CM interface.

## 3. Что не хватало и что я изменил

Для verbs-based workloads на Mellanox часто нужен не только `uverbsX`, но и
глобальный control device `rdma_cm`.

В этом срезе я сделал bounded prototype в `sdn`:

- [`handlers.go`](/Users/myskat_90/flant/aleksandr-podmoskovniy/sdn/images/agent/src/internal/dra-plugin/driver/handlers.go:263)
  теперь оппортунистически добавляет `/dev/infiniband/rdma_cm` в CDI device
  nodes, если:
  - claim уже использует InfiniBand verbs files;
  - такой device существует на host.
- Обновил краткое описание режима в
  [docs/README.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/sdn/docs/README.md:63)
  и
  [docs/README.ru.md](/Users/myskat_90/flant/aleksandr-podmoskovniy/sdn/docs/README.ru.md:63),
  чтобы зафиксировать Mellanox `DPDK` path как experimental RDMA/verbs-capable
  baseline.

Это не вводит новый API и не ломает текущую модель `NetDev` / `VFIO-PCI` /
`DPDK`.

## 4. Почему я не стал вводить отдельный `bindingMode: RDMA`

Отдельный RDMA mode уже не является "маленькой" правкой. Он потребовал бы:

- расширить API/CRD enum;
- договориться, чем `RDMA` семантически отличается от Mellanox `DPDK`, если
  реальный kernel driver в обоих случаях `mlx5_core`;
- научиться стабильно отражать этот режим в status/discovery, а не только в
  desired spec;
- определить exact mount contract:
  - только `uverbs` + `rdma_cm`;
  - или ещё `issm/umad`;
  - нужен ли отдельный sysfs/class contract.

Итого: отдельный `RDMA` binding mode я считаю отдельным follow-up workstream, а
не безопасным incidental change.

## 5. Практический verdict по RDMA

Если цель — как можно быстрее проверить RDMA на текущем `sdn`, то defendable
путь такой:

- Mellanox only;
- `UnderlayNetwork` + `bindingMode: DPDK`;
- `Shared` mode для VF-based isolation или `Dedicated` для целого PF;
- текущий prototype даёт `uverbs*` + `rdma_cm` + netdev + sysfs;
- на узлах отдельно должны быть готовы RDMA stack / OFED prerequisites.

Если цель — GPUDirect RDMA, этого недостаточно само по себе. Нужны ещё:

- совместимая GPU/NIC topology;
- host-side GPU driver integration (`nvidia-peermem` / equivalent stack);
- корректный RDMA driver stack на host.

### Live finding on `k8s-dvp.apiac.ru`

На `k8s-dvp-w1-gpu.apiac.ru` `nvidia-peermem` загрузился штатно:

- в `/proc/kallsyms` есть
  `ib_register_peer_memory_client` / `ib_unregister_peer_memory_client`;
- `modprobe nvidia-peermem` проходит;
- `lsmod` показывает `nvidia_peermem`.

На `k8s-dvp-w3-gpu.apiac.ru` изначально до host-side OFED bootstrap ситуация
была другой:

- `nvidia-peermem.ko` физически есть;
- `modprobe nvidia-peermem` падает с `Invalid argument`;
- в `/proc/kallsyms` нет
  `ib_register_peer_memory_client` / `ib_unregister_peer_memory_client`;
- внутри самого `nvidia-peermem.ko` тоже нет ссылок на эти peer-memory symbols.

Практический вывод на тот момент:

- на `w3` проблема не в том, что "модуль забыли включить";
- текущий `kernel/RDMA/GPU` stack на `w3` не даёт `nvidia-peermem` зацепиться
  за legacy peer-memory API;
- для `V100` это критично, потому что realistic GPUDirect path здесь именно
  legacy `nvidia-peermem`, а не modern DMA-BUF path.

Отсюда текущий best-effort install path для `w3`:

1. Не пытаться чинить это только через `/etc/modules-load.d`.
2. Поставить Mellanox OFED stack с peer-memory support.
3. После OFED пересобрать/переустановить NVIDIA driver, чтобы
   `nvidia-peermem` собрался уже против нужного RDMA stack.
4. После этого проверять:
   - `grep -E '\\bib_register_peer_memory_client\\b' /proc/kallsyms`
   - `modprobe nvidia-peermem`
   - `lsmod | grep nvidia_peermem`

Для `RED OS 8` с кастомным kernel `6.12` наиболее defendable путь сейчас не
DOCA packaged path, а `MLNX_OFED` source/install path. По свежим NVIDIA docs
community/custom kernels требуют source build, а `doca-kernel-support` не
поддерживает customized/unofficial kernels.

### Live blocker on `w3`: `mlnx-ofa_kernel` build vs RED OS rpm hardening

При реальной попытке собрать `MLNX_OFED_SRC-24.10-3.2.5.0` на
`k8s-dvp-w3-gpu.apiac.ru` после ручного `--distro rhel8.10` и отключения
`brp-mangle-shebangs` installer дошёл до `mlnx-ofa_kernel`, но упал уже на
сборке kernel modules:

- kernel собран компилятором `gcc 12.4.1` (`RED SOFT 12.4.0-1`);
- на узле дефолтный `gcc` = `13.3.1`;
- `rpmbuild` на RED OS тянет `%{optflags}` из `/usr/lib/rpm/redsoft/macros`;
- эти flags включают
  `-specs=/usr/lib/rpm/redsoft/redsoft-hardened-cc1`, который принудительно
  добавляет `-fPIE`;
- для external kernel module build это ломается на
  `cc1: error: code model kernel does not support PIC mode`.

Это важное уточнение: текущий блокер на `w3` уже не про "unsupported distro",
а про комбинацию:

- custom kernel build;
- mismatch compiler major (`gcc 13` vs kernel `gcc 12`);
- RED OS rpm hardening flags, которые уместны для userspace RPM, но не для
  kernel modules.

Практический next attempt для `w3` выглядит так:

1. Не полагаться на stock RED OS `%{optflags}` и `%{build_ldflags}`:
   - c native `gcc 13` build ломается на `-fPIE` из
     `redsoft-hardened-cc1` (`code model kernel does not support PIC mode`);
   - при переключении на `x86_64-linux-gnu-gcc` build доходит до `compat`,
     но сначала падает на `annobin.so`, потому что redsoft specs ищут plugin
     в `cross-gcc` layout
     (`/usr/lib/gcc/x86_64-linux-gnu/12/plugin/annobin.so`), а system
     `gcc-plugin-annobin` установлен только для native toolchain;
   - даже после вырезания `annobin/hardening` cross compiler остаётся
     unsuitable для этого места, потому что `compat/configure` должен
     собирать и линковать обычные host executables, а
     `x86_64-linux-gnu-gcc` на этом узле падает на
     `cannot find crt1.o`, `crti.o`, `-lc`, `crtn.o`.
2. Поэтому practical workaround должен подменять только `rpmbuild`,
   добавляя `--define` для:
   - `optflags`
   - `build_cflags`
   - `build_cxxflags`
   - `build_ldflags`
   чтобы вырезать hardening/annobin/LTO из RPM-side flags.
3. Запускать `install.pl` так, чтобы:
   - `rpmbuild` резолвился в wrapper, который прокидывает безопасные
     `--define`;
   - `gcc` оставался native `/usr/bin/gcc`, а не `x86_64-linux-gnu-gcc`.

Практический вывод по toolchain:

- cross packages `gcc-x86_64-linux-gnu-12.4.0-1.red80` /
  `gcc-c++-x86_64-linux-gnu-12.4.0-1.red80` на `w3` есть, но как глобальная
  подмена `gcc` для OFED install path они здесь не подходят;
- следующий defendable attempt на `w3` должен проверять именно комбинацию
  `native gcc + rpmbuild wrapper without redsoft hardening`.

Если после этого `mlnx-ofa_kernel` соберётся, уже имеет смысл повторно
проверять host-side peer-memory path и `modprobe nvidia-peermem` на `w3`.

### Live blocker on `w3`: `xpmem` build requires autotools

Следующая реальная попытка подтвердила, что workaround с native `gcc` и
wrapper вокруг `rpmbuild` был правильным:

- `mlnx-ofa_kernel`
- `mlnx-ofa_kernel-modules`
- `mlnx-ofa_kernel-devel`
- `mlnx-ofa_kernel-source`
- `knem`
- `knem-modules`

уже собрались и установились.

Новый blocker теперь другой и существенно уже по области: сборка `xpmem`
падает не на kernel ABI и не на RED OS hardening, а на отсутствующем
build-time tool:

- `./autogen.sh: line 3: autoreconf: command not found`

Практический вывод:

- текущий стоп на `w3` уже не про `mlnx-ofa_kernel`;
- для продолжения нужен autotools userspace stack, как минимум пакет,
  который даёт `autoreconf`;
- следующий attempt должен начинаться с установки build dependency вроде
  `autoconf`, а по-хорошему сразу полного минимального набора autotools
  (`autoconf`, `automake`, `libtool`), после чего стоит повторить тот же
  `install.pl`.

Это уже похоже на обычный source-build dependency gap, а не на новый
архитектурный blocker по kernel/RDMA integration.

Следующая попытка после установки autotools показала ещё одно важное
уточнение:

- `autoreconf` уже проходит;
- но `xpmem` всё равно падает на `configure: C compiler cannot create executables`;
- в debug info видно, что `gcc --version` во время этого прогона снова был
  `x86_64-linux-gnu-gcc`, а не native `/usr/bin/gcc`.

Практический вывод:

- в `PATH=/root/ofed-wrap/bin:$PATH` на `w3` всё ещё оставался старый wrapper
  для `gcc`/`g++`;
- этот wrapper снова перевёл OFED build на cross compiler, хотя для
  `compat/configure` и `xpmem/configure` нужен обычный host compiler;
- перед следующим прогоном нужно оставить в `/root/ofed-wrap/bin` только
  wrapper для `rpmbuild`, а wrappers для `gcc` и `g++` удалить.

В этом же логе видно ещё один штрих: `xpmem` build script патчит `ltmain.sh`
и подмешивает `-specs=/usr/lib/rpm/redsoft/redsoft-hardened-ld`. Это важно
держать в уме, но по текущему логу first fix всё равно не в этом, а в
возврате на native `gcc`.

### Live blocker on `w3`: `rdma-core` build requires `cmake`

После возврата на native `gcc` и повторного прогона OFED install path на `w3`
ситуация двинулась дальше:

- `xpmem` и `xpmem-modules` уже собрались и установились;
- `kernel-mft`, `iser`, `srp`, `isert`, `mlnx-nvme` тоже дошли до install;
- новый stop находится уже в userspace пакете `rdma-core`.

Текущий first blocker по логу:

- `%build` падает на
  `/var/tmp/rpm-tmp.gUhyio: line 50: cmake: command not found`

Практический вывод:

- текущий стоп уже не про kernel modules и не про `xpmem`;
- на узле есть `cmake-filesystem`, но нет самого `cmake` binary;
- ближайший следующий шаг на `w3` — поставить `cmake` и повторить тот же
  `install.pl`.

Надо ожидать, что при `--without-depcheck` после `cmake` могут всплыть ещё
следующие userspace build deps для `rdma-core`/`pyverbs`, но прямо сейчас
first actionable blocker именно отсутствующий `cmake`.

### Live blocker on `w3`: `rdma-core` configure now stops on missing `libnl` devel

После установки `cmake` тот же `rdma-core` build path продвинулся ещё дальше:

- `cmake` уже стартует и определяет native `gcc 13.3.1`;
- warning про `pandoc` и `rst2man` остаётся non-fatal;
- warning про `pyverbs build requested but python development files not found`
  пока тоже не является текущим first stop.

Новый first blocker по логу:

- `pkg-config` не находит `libnl-3.0` и `libnl-route-3.0`;
- configure валится на
  `The following required packages were not found: libnl-3.0, libnl-route-3.0`

Практический вывод:

- на узле уже стоят runtime пакеты `libnl3` и `libnl3-cli`, но отсутствует
  `libnl3-devel`, который и даёт `.pc` files для `pkg-config`;
- ближайший следующий шаг на `w3` — поставить `libnl3-devel` и повторить тот
  же `install.pl`;
- чтобы не тратить ещё один rerun только на `pyverbs`, разумно сразу добавить
  и `python3-devel`: текущий лог уже показывает, что python development
  headers/import libs на узле отсутствуют.

### Live blocker on `w3`: `rdma-core` packaging expects `rdma-ndd`, but buildroot does not contain it

После установки `libnl3-devel` и `python3-devel` `rdma-core` ушёл ещё дальше:

- сам build/install phase уже проходит;
- `BUILDROOT` наполняется библиотеками, утилитами, man pages и `ibacm`/`srp_daemon`;
- stop происходит уже не в compile/configure, а на rpm `%files` этапе.

Текущий first blocker по логу:

- rpm processing падает на отсутствующих файлах:
  `60-rdma-ndd.rules`, `rdma-ndd`, `rdma-ndd.service`,
  `man8/rdma-ndd.*`

Практический вывод:

- это уже packaging mismatch, а не обычный missing dependency;
- upstream `rdma-core` добавляет subdir `rdma-ndd` только при `UDEV_FOUND`,
  а spec при этом всё равно ожидает `rdma-ndd` artifacts в `%files`;
- наиболее вероятная причина на `w3` — в build environment всё ещё нет
  development package, который даёт `libudev` headers/pkg-config metadata;
- для RHEL-like систем это обычно `systemd-devel`, потому что `libudev`
  headers поставляются именно им.

Практический следующий шаг:

- поставить пакет, который даёт `pkgconfig(libudev)` и `libudev.h`
  (наиболее вероятно `systemd-devel`);
- потом повторить тот же `install.pl`.

Если после установки `systemd-devel` `rdma-ndd` всё равно не появится, тогда
это уже не dependency gap, а OFED/redhat spec mismatch, и следующий шаг будет
не rerun вслепую, а inspection `rdma-core.spec` и cmake summary на предмет
`UDEV_FOUND`.

### Live blocker on `w3`: `rdma-ndd` fixed, but `python3-pyverbs` subpackage is empty

После установки `systemd-devel` `rdma-core` ушёл ещё дальше:

- `rdma-ndd` теперь реально собирается и попадает в `BUILDROOT`;
- `%files` для `rdma-ndd` больше не является blocker;
- rpm processing успевает обработать `rdma-core`, `rdma-core-devel`,
  `infiniband-diags`, `libibverbs`, `ibacm`, `librdmacm`, `srp_daemon`;
- новый stop происходит уже на subpackage `python3-pyverbs`.

Текущий first blocker по логу:

- отсутствует каталог
  `/usr/lib64/python3.11/site-packages/pyverbs`;
- отсутствует `%doc` содержимое
  `/usr/share/doc/python3-pyverbs/tests`.

Практический вывод:

- это уже не `libudev`/`rdma-ndd` проблема;
- `install.pl` по-прежнему идёт с `--without-depcheck`, а `rpmbuild` запускается
  с `--nodeps`, поэтому missing build requirements для pyverbs не подтягиваются;
- upstream `rdma-core` для `with_pyverbs` ожидает как минимум
  `python3-Cython` и `python3-docutils`; в README upstream также фигурирует
  `pandoc` как часть нормального build environment;
- по package dump в логе на `w3` уже есть `python3-devel`, но не видно
  `python3-Cython`, `python3-docutils` и `pandoc`.

Практический следующий шаг:

- поставить `python3-Cython`, `python3-docutils` и `pandoc`;
- потом повторить тот же `install.pl`.

Если после этого `python3-pyverbs` всё равно останется пустым, следующий шаг —
не blind rerun, а inspection pyverbs install step/spec logic: почему включён
subpackage `python3-pyverbs`, но install phase не кладёт `site-packages/pyverbs`.

### Live blocker on `w3`: `pyverbs` fixed, next stop is `perftest` missing `pciutils-devel`

Следующий лог показывает, что после установки `python3-Cython`,
`python3-docutils` и `pandoc` сборка ушла дальше:

- `rdma-core` пакеты уже успешно собраны и установлены;
- `python3-pyverbs` больше не является first blocker;
- следующий stop происходит на `perftest-24.10.0`.

Текущий first blocker по логу `perftest-24.10.0.rpmbuild.log`:

- `configure` доходит до проверки `pci/pci.h`;
- затем падает с явным сообщением:
  `configure: error: pciutils header files not found, consider installing pciutils-devel`.

Практический вывод:

- это обычный следующий dependency gap из-за `--without-depcheck` и
  `rpmbuild --nodeps`;
- runtime `pciutils`/`pciutils-libs` на узле уже есть, но dev headers отсутствуют;
- следующий обязательный пакет — именно `pciutils-devel`.

Практический следующий шаг:

- поставить `pciutils-devel`;
- потом повторить тот же `install.pl`.

Побочный шум в логе:

- warning `invalid host type: /usr/bin/make` и промежуточное сообщение
  `No such file or directory` в autotools/libtool patch block выглядят
  подозрительно, но не являются текущим first blocker, потому что `configure`
  продолжает работу и реально падает уже на отсутствии `pci/pci.h`.

### Live blocker on `w3`: `perftest` fixed, current stop is `mpitests` in `osu-micro-benchmarks`

Следующий лог показывает, что после установки `pciutils-devel` сборка ушла ещё
дальше:

- `perftest` уже собрался;
- `openmpi-4.1.7rc1` и связанные runtime packages уже стоят на узле;
- новый stop происходит на `mpitests-3.2.24`.

Текущий first blocker по логу `mpitests-3.2.24.rpmbuild.log`:

- Intel MPI Benchmarks (`imb/src`) собираются до конца, там только warnings;
- падение начинается на `osu-micro-benchmarks`:
  `./configure --prefix=... CC=/usr/mpi/gcc/openmpi-4.1.7rc1/bin/mpicc ...`;
- `configure` падает на базовом autoconf runtime check:
  `configure: error: cannot run C compiled programs`;
- последующий `c/openshmem` failure (`No targets specified and no makefile found`)
  — уже downstream noise после неуспешного `configure`.

Практический вывод:

- это уже не очередной missing `*-devel` пакет, а runtime execution problem для
  тестовой программы, собранной через `mpicc`;
- по одному `rpmbuild.log` точную причину назвать нельзя: нужен текст из
  `osu-micro-benchmarks/config.log`, где autoconf пишет, чем именно закончился
  запуск `./conftest`;
- наиболее вероятный класс причин — dynamic loader / library path mismatch для
  бинаря, собранного через `/usr/mpi/gcc/openmpi-4.1.7rc1/bin/mpicc`.

Практический следующий шаг:

- смотреть `config.log` именно внутри
  `/var/tmp/OFED_topdir/BUILD/mpitests-3.2.24/osu-micro-benchmarks/`;
- отдельно вручную собрать и запустить минимальный бинарь через тот же `mpicc`
  под тем же `LD_LIBRARY_PATH`, чтобы получить точную runtime ошибку.

Минимальная диагностическая последовательность:

- `cd /var/tmp/OFED_topdir/BUILD/mpitests-3.2.24/osu-micro-benchmarks`
- `grep -n -A20 -B20 "cannot run C compiled programs" config.log`
- `printf 'int main(void){return 0;}\n' >/tmp/ompi-smoke.c`
- `env LD_LIBRARY_PATH=/usr/mpi/gcc/openmpi-4.1.7rc1/lib64:/usr/mpi/gcc/openmpi-4.1.7rc1/lib /usr/mpi/gcc/openmpi-4.1.7rc1/bin/mpicc /tmp/ompi-smoke.c -o /tmp/ompi-smoke`
- `ldd /tmp/ompi-smoke`
- `env LD_LIBRARY_PATH=/usr/mpi/gcc/openmpi-4.1.7rc1/lib64:/usr/mpi/gcc/openmpi-4.1.7rc1/lib /tmp/ompi-smoke`

Уточнение после просмотра `config.log`:

- верхнеуровневое `cannot run C compiled programs` вводит в заблуждение;
- реальная первичная ошибка случается раньше, на линковке `conftest`:
  `/usr/bin/ld: relocation R_X86_64_32 against '.rodata' can not be used when making a PIE object; recompile with -fPIE`;
- затем `configure` пытается запустить `./conftest`, но файла уже нет, потому что
  link step завершился ошибкой;
- это выглядит как mismatch между hardening `LDFLAGS` от RED OS и тем, как
  `osu-micro-benchmarks` вызывает `mpicc` без совместимых `CFLAGS`;
- дополнительный сигнал в `config.log`: `ac_cv_env_CFLAGS_value=` пустой, то есть
  в этот `configure` реальные build `CFLAGS` не доезжают, тогда как hardening
  linker flags применяются.

Практический рабочий вывод:

- first fix нужен не в `LD_LIBRARY_PATH`, а в build flags для
  `osu-micro-benchmarks`;
- нужно либо явно протащить `CFLAGS/CXXFLAGS` в его `./configure`, либо локально
  ослабить hardening/PIE требования для этого sub-build, если upstream mpitests
  не дружит с текущим RED OS toolchain policy.

Подтверждение ручным reproduce на `w3`:

- запуск `./configure` внутри
  `/var/tmp/OFED_topdir/BUILD/mpitests-3.2.24/osu-micro-benchmarks` с
  `CC=/usr/mpi/gcc/openmpi-4.1.7rc1/bin/mpicc`,
  `CXX=/usr/mpi/gcc/openmpi-4.1.7rc1/bin/mpicxx`,
  `CFLAGS='-O2 -fPIE'`, `CXXFLAGS='-O2 -fPIE'` и теми же hardening `LDFLAGS`
  проходит успешно и доходит до `config.status`;
- это подтверждает, что среда хоста и установленный OpenMPI уже достаточны для
  данного шага, а проблема сидит именно в packaging/build invocation
  `mpitests`, где для `osu-micro-benchmarks` теряются совместимые compile flags;
- следующий фикс нужен в spec/Makefile/обвязке установки MLNX_OFED:
  вызов `osu-micro-benchmarks ./configure` должен явно получать
  `CFLAGS/CXXFLAGS/LDFLAGS`, а `CXX` лучше выставлять в `mpicxx`, а не
  `mpicc`.

Дополнительное подтверждение:

- после успешного `./configure` ручной `make MPIHOME=/usr/mpi/gcc/openmpi-4.1.7rc1`
  внутри `osu-micro-benchmarks` также проходит;
- отдельный `make -C c/openshmem` через `oshcc` тоже проходит;
- следовательно, текущий `mpitests` blocker целиком воспроизводится как
  packaging bug в MLNX_OFED build recipe, а не как defect установленного
  OpenMPI, `oshcc`, `openshmem` или runtime-библиотек на хосте;
- практический short-term workaround для ручной сборки: перед вызовом проблемного
  sub-build гарантированно выставлять
  `CFLAGS='-O2 -fPIE'`, `CXXFLAGS='-O2 -fPIE'`, `LDFLAGS='<redsoft hardening>'`,
  `CC=.../mpicc`, `CXX=.../mpicxx`;
- правильный fix для повторяемой установки: патчить spec или make recipe так,
  чтобы эти флаги и `CXX` передавались в `osu-micro-benchmarks` автоматически.

Уточнение после повторного запуска `./install.pl`:

- `install.pl` теперь доходит до `Build mpitests_openmpi 3.2.24 RPM`; все
  предыдущие blockers уже сняты;
- в `%build` логе видно, что recipe вызывает:
  `./configure --prefix=/usr/mpi/gcc/openmpi-4.1.7rc1/tests/osu-micro-benchmarks CC=/usr/mpi/gcc/openmpi-4.1.7rc1/bin/mpicc CXX=/usr/mpi/gcc/openmpi-4.1.7rc1/bin/mpicc && make MPIHOME=...`;
- то есть MLNX_OFED recipe по-прежнему использует неправильный `CXX`
  (`mpicc` вместо `mpicxx`) и не пробрасывает `CFLAGS/CXXFLAGS/LDFLAGS`
  непосредственно в проблемный `./configure` вызов;
- ручной `configure` и `make` проходят только когда эти значения задаются явно;
- итоговый first fix для полностью зелёной установки: править именно
  `mpitests` build recipe/SRPM, а не системные пакеты на хосте.

Практический runbook для ручной пересборки только `mpitests_openmpi`:

- вынести SRPM в отдельный `_topdir`, например `/root/rpmbuild-mpitests`, чтобы
  не зависеть от временного дерева `install.pl`;
- распаковать `mpitests-3.2.24-2ffc2d6.2410068.src.rpm` и править
  `SPECS/mpitests.spec`;
- в spec заменить проблемный вызов `osu-micro-benchmarks ./configure` так,
  чтобы он использовал `CXX=%{path_to_mpihome}/bin/mpicxx` и явно передавал
  `CFLAGS`, `CXXFLAGS`, `LDFLAGS`;
- затем собирать `rpmbuild -bb` с теми же define-параметрами, которые использует
  `install.pl`, включая `path_to_mpihome=/usr/mpi/gcc/openmpi-4.1.7rc1`;
- устанавливать уже только получившийся `mpitests_openmpi` RPM поверх текущего
  хоста, без повторного полного `./install.pl`.

Практический fallback, если нужен не rebuild, а готовый reusable RPM:

- через `kubectl debug node/...` на текущем `k8s-main` удалось подтвердить, что
  доступная RED OS нода `k8s-w3-gpu.apiac.ru` не является тем же хостом, что
  `k8s-dvp-w3-gpu`, и на ней нет дерева `MLNX_OFED_SRC-24.10-3.2.5.0`;
- вместо bootstrap полного build environment найден официальный уже собранный
  пакет NVIDIA/Mellanox:
  `mpitests_openmpi-3.2.24-2ffc2d6.2410068.x86_64.rpm`
  из `latest-24.10/rhel8.10/x86_64`;
- metadata пакета совпадает с целевым артефактом из лога:
  - `Name: mpitests_openmpi`
  - `Version: 3.2.24`
  - `Release: 2ffc2d6.2410068`
  - `Source RPM: mpitests_openmpi-3.2.24-2ffc2d6.2410068.src.rpm`;
- runtime requirements у готового RPM узкие:
  `libmpi.so.40`, `liboshmem.so.40`, `libc`, `libm`, `libpthread`;
- это делает пакет практическим workaround для других RED OS 8 хостов с уже
  установленным совместимым `openmpi 4.1.7rc1` из того же OFED release;
- зафиксированный checksum проверенного файла:
  `993e629f7dd7d90834dd21580de69517f6f6f5af8a9682d99353bb50d6f0625c`.

Финальный практический результат на правильном `dvp`-хосте:

- после переключения kube context на `k8s-dvp.apiac.ru` удалось зайти на
  `k8s-dvp-w3-gpu.apiac.ru` через `kubectl debug node/...` в `kube-system` и
  выполнить rebuild прямо на целевом RED OS 8.0.2 хосте;
- на хосте были доступны:
  - `/root/MLNX_OFED_SRC-24.10-3.2.5.0`
  - `mpitests-3.2.24-2ffc2d6.2410068.src.rpm`
  - весь ранее доустановленный build toolchain;
- rebuild был выполнен в отдельном topdir:
  `/root/rpmbuild-mpitests-fixed`;
- для успешной сборки понадобились два локальных packaging fix:
  - в `mpitests-3.2.24/Makefile` заменить `CXX=$(MPIHOME)/bin/mpicc` на
    `CXX=$(MPIHOME)/bin/mpicxx`;
  - передавать в проблемный `osu-micro-benchmarks ./configure` явные
    `CFLAGS="-O2 -fPIE"`, `CXXFLAGS="-O2 -fPIE"` и RED OS hardening `LDFLAGS`;
- после этого сам `%build` прошёл, но `%install` упирался в `check-rpaths` на
  `runpath '/usr/mpi/gcc/openmpi-4.1.7rc1/lib64'`;
- для получения пригодного RPM пришлось запускать `rpmbuild` с
  `QA_RPATHS=0x0002`, что переводит этот gate из fatal error в warning;
- итоговый RPM успешно собран:
  `/root/rpmbuild-mpitests-fixed/RPMS/x86_64/mpitests_openmpi-3.2.24-2ffc2d6.2410068.x86_64.rpm`;
- пакет установлен на `k8s-dvp-w3-gpu.apiac.ru` и подтверждается через
  `rpm -q mpitests_openmpi`;
- локальная копия сохранена как reusable artifact:
  `/Users/myskat_90/Downloads/mlnx-ofed-rpms/mpitests_openmpi-3.2.24-2ffc2d6.2410068.x86_64.rpm`;
- checksum реально собранного и проверенного локального файла:
  `f4af63ffbfce24a0333ad831a6a4b8771ba5dc90877068cfef79fdf5d458734e`.

## 6. Что делать для будущего `GPU + RDMA NIC` matching

### Базовый вывод

Чистый DRA хорошо решает:

- публикацию devices;
- per-pod claims;
- device filtering по attributes/CEL;
- fallback selection через prioritized subrequests;
- логические/composite devices через partitionable devices.

Но если GPU и NIC остаются двумя независимыми драйверами/классами, то одного
DRA недостаточно, чтобы надёжно гарантировать "эта GPU должна ехать именно с
этим NIC по topology/pairing policy".

### Что я рекомендую

Предпочтительный порядок решений:

1. Сначала попытаться выразить pairing внутри resource model, а не во внешнем
   extender.
   - Вариант A: один higher-level driver публикует composite `GPU+NIC bundle`
     devices.
   - Вариант B: один coordinating allocator формирует logical devices/claims
     поверх двух inventories.
2. Если этого недостаточно, использовать не scheduler extender, а
   scheduler framework plugin или отдельный secondary scheduler.
3. Topology Manager на узле включать всё равно:
   - `pod` scope;
   - `single-numa-node` или как минимум `restricted`, если workload реально
     latency-sensitive.

### Почему не extender-first

- upstream scheduler extender умеет только filter/prioritize;
- reserve / prebind / richer in-tree extension points через webhook недоступны;
- если логика паринга GPU+NIC станет stateful и topology-heavy, extender быстро
  упрётся в собственные границы.

## 7. Live GPUDirect readiness check after RED OS OFED bootstrap

После того как на `k8s-dvp-w3-gpu.apiac.ru` был доведён до usable состояния
host-side OFED/OpenMPI stack, отдельно проверил именно prerequisites для
`GPUDirect RDMA`.

Изначальная проблема на `w3` после OFED bootstrap выглядела так:

- `nvidia-smi -L` видит `Tesla V100-PCIE-32GB`;
- `rpm -q perftest openmpi mpitests_openmpi` подтверждает, что host-side RDMA
  userland уже установлен;
- `modinfo nvidia-peermem` показывает модуль
  `/lib/modules/6.12.37-1.red80.x86_64/kernel/drivers/video/nvidia-peermem.ko`;
- `grep -E '\\bib_register_peer_memory_client\\b|\\bib_unregister_peer_memory_client\\b' /proc/kallsyms`
  уже показывает
  `ib_register_peer_memory_client` /
  `ib_unregister_peer_memory_client`;
- но `modprobe nvidia-peermem` падает с `Invalid argument`;
- `nm -u /lib/modules/$(uname -r)/kernel/drivers/video/nvidia-peermem.ko`
  не показывает ссылок на
  `ib_register_peer_memory_client` /
  `ib_unregister_peer_memory_client`;
- это означало, что текущий `nvidia-peermem.ko` был собран ещё до появления
  OFED peer-memory symbols и его нужно было пересобрать, а не просто
  перепробовать `modprobe`.

Рабочий remediation path на `w3`:

- на host уже был `.run`-инсталл NVIDIA
  `NVIDIA-Linux-x86_64-580.76.05.run`, а не RPM/DKMS;
- после установки OFED тот же installer был заново запущен на host через
  `kubectl debug node/...` с параметрами:

  ```bash
  sh ./NVIDIA-Linux-x86_64-580.76.05.run \
    --skip-module-load \
    --no-dkms \
    --kernel-module-type=proprietary
  ```

- installer пересобрал kernel modules уже против текущего OFED stack;
- после этого SHA256 `nvidia-peermem.ko` изменился:
  - до пересборки:
    `9e850bae95c943be1b3d0d76e3fa02385a97fc9781bdb7dde9fc6d83efce62ac`
  - после пересборки:
    `38672c398bd2544826df830f0583fb16ee02b7f2da5f0f028aaaf40b1e8c49f7`
- новый `nvidia-peermem.ko` уже содержит undefined references на
  `ib_register_peer_memory_client` /
  `ib_unregister_peer_memory_client`;
- `modprobe nvidia-peermem` теперь проходит;
- `lsmod` показывает:
  - `nvidia_peermem`
  - `mlx5_ib`
  - `ib_uverbs`
  - `ib_core`
- в `dmesg` есть normal load messages от `nvidia_peermem`, без `Invalid argument`;
- для автозагрузки создан
  `/etc/modules-load.d/nvidia-peermem.conf` со строкой `nvidia-peermem`.
- сам NVIDIA installer в конце всё равно рекомендовал reboot, потому что core
  NVIDIA modules были загружены во время переустановки; для текущей проверки
  это не помешало, но как maintenance path safer считать planned reboot.

Текущее состояние на `w3`:

- `host RDMA`: yes;
- `GPUDirect RDMA prerequisite`: yes;
- после reboot ноды `nvidia_peermem` автозагрузился и остался в `lsmod`, то
  есть persistence через `/etc/modules-load.d/nvidia-peermem.conf` подтверждена;
- stock `perftest` из OFED всё ещё был без CUDA support, потому что пакет был
  собран без user-space `cuda.h`.

Отдельный remediation path для `perftest` на `w3`:

- spec `perftest.spec` уже умеет CUDA через
  `%configure CUDA_H_PATH=%{_cuda_h_path}`;
- на `w3` был только kernel header `/usr/include/linux/cuda.h`, он для этого не
  подходит;
- минимально нужный user-space `cuda.h` был взят с `w1` из
  `/root/venv/lib/python3.10/site-packages/nvidia/cuda_runtime/include/cuda.h`
  и скопирован на `w3` как `/root/w1-cuda.h`;
- после этого тот же SRPM был пересобран на `w3` так:

  ```bash
  rpmbuild -bb --nodeps \
    --define "_topdir /root/rpmbuild-perftest-cuda" \
    --define "_cuda_h_path /root/w1-cuda.h" \
    /root/rpmbuild-perftest-cuda/SPECS/perftest.spec
  ```

- установка на `w3` шла так:

  ```bash
  rpm -Uvh --replacepkgs --nodeps \
    /root/rpmbuild-perftest-cuda/RPMS/x86_64/perftest-24.10.0-0.95.g370212b.2410325.x86_64.rpm
  ```

- `--nodeps` понадобился не потому, что runtime broken, а потому что NVIDIA
  driver на `w3` поставлен через `.run`, поэтому `rpm` database не знает
  provider для `libcuda.so.1`;
- после установки `/usr/bin/ib_write_bw --help` уже показывает:
  - `--use_cuda`
  - `--use_cuda_bus_id`
  - `--use_cuda_dmabuf`
- локальный reusable artifact сохранён как
  `/Users/myskat_90/Downloads/mlnx-ofed-rpms/perftest-24.10.0-0.95.g370212b.2410325.cuda.x86_64.rpm`
  с SHA256
  `e0fb76a38d02c424a8fdd84df631f64c0abde910041e7427c8b01ffeb2f28f24`.

Для контраста на `k8s-dvp-w1-gpu.apiac.ru`:

- `modprobe nvidia-peermem` проходит;
- `lsmod` показывает `nvidia_peermem`;
- в `/proc/kallsyms` есть
  `ib_register_peer_memory_client` /
  `ib_unregister_peer_memory_client`.

Практический вывод:

- на `w3` host-side prerequisite для `GPUDirect RDMA` уже закрыт;
- CUDA-enabled `perftest` на `w3` уже получен.

Поэтому для `w3` честный current verdict такой:

- `host RDMA`: yes;
- `GPUDirect RDMA prerequisite`: yes;
- `functional GPU-memory RDMA proof`: yes.

После этого был прогнан реальный межнодовый smoke между
`k8s-dvp-w1-gpu.apiac.ru` и `k8s-dvp-w3-gpu.apiac.ru`.

Для этого на `w1` дополнительно пришлось:

- доставить source tarball `perftest-24.10.0-0.95.g370212b.tar.gz`;
- собрать локальный CUDA-aware binary, потому что host-side `ib_write_bw` на
  `w1` изначально отсутствовал;
- использовать user-space CUDA header, который уже был на `w1`:
  `/root/venv/lib/python3.10/site-packages/nvidia/cuda_runtime/include/cuda.h`.

Минимальный build path на `w1` выглядел так:

```bash
mkdir -p /root/perftest-cuda-build-w1
tar -xf /root/perftest-24.10.0-0.95.g370212b.tar.gz -C /root/perftest-cuda-build-w1
cd /root/perftest-cuda-build-w1/perftest-24.10.0
CUDA_H_PATH=/root/venv/lib/python3.10/site-packages/nvidia/cuda_runtime/include/cuda.h ./configure
make -j2
./ib_write_bw --help | egrep -i 'use_cuda|cuda_dmabuf|cuda_bus_id'
```

На обоих host runtime parameters для теста оказались такими:

- device: `mlx5_0`;
- `GID index`: `1`;
- control IP:
  - `w1`: `192.168.3.60`
  - `w3`: `192.168.3.32`

Фактический двусторонний smoke:

- `w3 -> w1`

  ```bash
  # w1 server
  /root/perftest-cuda-build-w1/perftest-24.10.0/ib_write_bw \
    -d mlx5_0 -x 1 -m 4096 -F --report_gbits -D 5 --use_cuda=0

  # w3 client
  ib_write_bw \
    -d mlx5_0 -x 1 -m 4096 -F --report_gbits -D 5 --use_cuda=0 \
    192.168.3.60
  ```

  Результат:
  - `RC=0`
  - `BW average 56.99 Gb/sec`
  - `#bytes 65536`

- `w1 -> w3`

  ```bash
  # w3 server
  ib_write_bw \
    -d mlx5_0 -x 1 -m 4096 -F --report_gbits -D 5 --use_cuda=0

  # w1 client
  /root/perftest-cuda-build-w1/perftest-24.10.0/ib_write_bw \
    -d mlx5_0 -x 1 -m 4096 -F --report_gbits -D 5 --use_cuda=0 \
    192.168.3.32
  ```

  Результат:
  - `RC=0`
  - `BW average 56.80 Gb/sec`
  - `#bytes 65536`

Итоговый practical verdict:

- `k8s-dvp-w3-gpu.apiac.ru` после OFED bootstrap и пересборки
  `nvidia-peermem`/`perftest` действительно готов к `GPUDirect RDMA`;
- проблема на `w3` была не в железе и не в самом OFED runtime, а в
  host-specific bootstrap и packaging gaps;
- после их устранения межнодовый `ib_write_bw --use_cuda=0` на `w1`/`w3`
  проходит в обе стороны;
- временные `node-debugger` pod после прогона удалены.

### Performance tuning follow-up

После initial functional proof стало видно, что plain
`ib_write_bw --use_cuda=0` сам по себе не даёт line-rate:

- `w3 -> w1`, `qps=1`: `56.90 Gb/sec`
- `w1 -> w3`, `qps=1`: `56.80 Gb/sec`

Это оказалось не fabric ceiling, а benchmark/profile issue.

Сначала был снят fabric baseline без GPU memory:

```bash
ib_write_bw -d mlx5_0 -x 1 -m 4096 -F --report_gbits -D 10 192.168.3.60
```

Он дал `86.15 Gb/sec`, то есть сам RDMA path на host уже близок к line-rate.

Дальше был проверен `GPU/NIC` topology:

- на `w1`:
  - `mlx5_0` = `100 Gb/sec`
  - `mlx5_1` = `100 Gb/sec`
  - `nvidia-smi topo -m` показал:
    - `GPU0 <-> mlx5_0 = SYS`
    - `GPU1 <-> mlx5_0 = PHB`
    - `GPU0 <-> mlx5_1 = PHB`
- на `w3`:
  - `mlx5_0 = 100 Gb/sec`
  - `GPU0 <-> mlx5_0 = PHB`

Отсюда правильная pairing для теста `mlx5_0 <-> w3`:

- `w1`: `GPU1`, `PCI BUS ID 00000000:C2:00.0`
- `w3`: `GPU0`, `PCI BUS ID 00000000:01:00.0`

После этого было подтверждено:

- correct `GPU/NIC` pairing нужен, но сам по себе не спасает `ib_write_bw`;
- `ib_write_bw` даже после pairing остаётся capped:
  - `qps=1`: `56.90 Gb/sec`
  - `qps=8`: `61.62 Gb/sec`
- `--use_cuda_dmabuf` на `V100` здесь не работает:
  - `DMA-BUF is not supported on this GPU`

Fastest practical GPUDirect profile на текущем стенде получился не на
`ib_write_bw`, а на `ib_read_bw`.

Рабочий near-line-rate benchmark:

```bash
# w1 server
/root/perftest-cuda-build-w1/perftest-24.10.0/ib_read_bw \
  -d mlx5_0 -x 1 -q 8 -m 4096 -F --report_gbits -D 10 \
  --use_cuda_bus_id=00000000:C2:00.0

# w3 client
ib_read_bw \
  -d mlx5_0 -x 1 -q 8 -m 4096 -F --report_gbits -D 10 \
  --use_cuda_bus_id=00000000:01:00.0 \
  192.168.3.60
```

Результат:

- `RC=0`
- `BW average 95.10 Gb/sec`
- `#bytes 65536`

То есть practical tuning verdict теперь такой:

- `GPUDirect RDMA` на этом стенде действительно можно довести до
  `~95 Gb/sec`;
- для этого нужно:
  - explicit `GPU/NIC` pairing по topology;
  - `--use_cuda_bus_id` вместо неявного GPU ordinal;
  - `ib_read_bw`;
  - `qps=8`.

Оставшийся asymmetry:

- reverse `ib_read_bw` (`w1` client -> `w3` server`) остаётся около
  `61.7 Gb/sec`;
- это выглядит как host-specific ceiling именно когда `w3` выступает remote GPU
  source;
- дополнительный low-level signal на `w3`:
  - в guest уже включены `intel_iommu=on iommu=pt`;
  - `dmesg` показывает `DMAR: No ATSR found`;
  - `dmesg` показывает `DMAR: IOMMU batching disallowed due to virtualization`;
- следовательно, unresolved limit уже не в OFED install path и не в generic
  RDMA connectivity, а в asymmetric VM/peer-memory behavior этой стороны.

### Secure pod contract without hostPath / privileged

Live `sdn-rdma-test` pods подтверждают, что pod-level RDMA smoke не требует:

- `hostPath`;
- `privileged: true`.

Текущий working pod contract:

- `UnderlayNetwork` annotation с `bindingMode: DPDK`;
- `ResourceClaimTemplate` для текущего smoke пока создаётся вручную, а дальше
  уже используется обычный `sdn`/DRA path;
- inside pod есть:
  - direct netdev (`enp193s0np0` на `w1`, `enp2s0np0` на `w3`);
  - `/dev/infiniband/uverbs0`;
  - verbs tools;
- securityContext:
  - `runAsUser: 0`
  - capabilities:
    - `NET_ADMIN`
    - `NET_RAW`
    - `IPC_LOCK`
- volumes:
  - только стандартный projected `serviceAccount` volume.

Что из capability set реально обязательно:

- `IPC_LOCK` — нужен для verbs benchmark;
- `NET_ADMIN` — нужен только если руками назначать временный `/30`;
- `NET_RAW` — нужен только для `ping`.

То есть для secure contour, где direct IP на интерфейс даст сам runtime/CNI,
minimal benchmark profile можно сжать до pod без `hostPath`, без `privileged` и
только с `IPC_LOCK`.

После reboot `w3` старый `rdma-pod-w3-00` ушёл в `Failed/Unknown`, но был
поднят заново тем же pod spec и снова стал `Ready`.

Pod-level near-line-rate verification на этих же условиях:

```bash
# w1 pod
kubectl -n sdn-rdma-test exec rdma-pod-w1-c0 -- \
  bash -lc 'ip addr replace 172.31.120.1/30 dev enp193s0np0'

# w3 pod
kubectl -n sdn-rdma-test exec rdma-pod-w3-00 -- \
  bash -lc 'ip addr replace 172.31.120.2/30 dev enp2s0np0'

# w1 server
kubectl -n sdn-rdma-test exec rdma-pod-w1-c0 -- \
  bash -lc "ib_read_bw -d mlx5_0 -x 3 -q 8 -m 4096 -F --report_gbits -D 10"

# w3 client
kubectl -n sdn-rdma-test exec rdma-pod-w3-00 -- \
  bash -lc "ib_read_bw -d mlx5_0 -x 3 -q 8 -m 4096 -F --report_gbits -D 10 172.31.120.1"
```

Результат:

- `RC=0`
- `BW average 97.22 Gb/sec`

Это не GPUDirect proof inside pod, потому что в этих validation pod пока нет
отдельного GPU claim/device. Но это уже подтверждает, что:

- near-line-rate RDMA внутри pod достижим;
- `sdn` underlay path работает без host filesystem mounts;
- для будущего pod-level GPUDirect остаётся добавить именно GPU DRA path, а не
  чинить сам underlay RDMA baseline.

## 13. Pod-level GPUDirect RDMA (`sdn` + GPU DRA) на `w1/w3`

### 13.1. Что было препятствием

Первый combined `GPU + RDMA` pod не поднимался по двум разным причинам:

1. auto-generated network `DeviceClass` от `sdn` имел selector без
   `device.driver == "network.deckhouse.io"` и scheduler падал на CEL runtime
   error:

   - `class d8-sdn-rdma-w1-pairc0: selector #0: CEL runtime error: no such key: underlayNetwork`
   - `class d8-sdn-rdma-w3-pair00: selector #0: CEL runtime error: no such key: underlayNetwork`

2. после временного manual workaround exact ресурсы уже были заняты:

   - `rdma-pod-w1-c0` держал `rdma-w1-pairc0`;
   - `rdma-pod-w3-00` держал `rdma-w3-pair00`;
   - `gpuonly-pod-w1-c2` держал GPU `00000000:C2:00.0`;
   - `gpuonly-pod-w3-01` держал GPU `00000000:01:00.0`.

Фактически combined pod начал schedulиться только после освобождения exact PF и
exact GPU пар.

### 13.2. Working temporary DRA objects

Exact network workaround:

- `DeviceClass manual-rdma-w1-pairc0`
- `DeviceClass manual-rdma-w3-pair00`

Их selector в рабочем виде:

```cel
device.driver == "network.deckhouse.io" &&
device.attributes["network.deckhouse.io"].underlayNetwork == "rdma-w1-pairc0"
```

и аналогично для `rdma-w3-pair00`.

GPU exact matching:

- `DeviceClass nvidia-v100-w1-c2-gpudirect`
- `DeviceClass nvidia-v100-w3-01-gpudirect`
- `ResourceClaimTemplate gpu-v100-w1-c2-gpudirect`
- `ResourceClaimTemplate gpu-v100-w3-01-gpudirect`

Проверенный fixed pairing:

- `w1`: `rdma-w1-pairc0` + GPU `00000000:C2:00.0`
- `w3`: `rdma-w3-pair00` + GPU `00000000:01:00.0`

### 13.3. Combined pod state

После освобождения старых resource holders pod'ы:

- `gpudirect-pod-w1-c2`
- `gpudirect-pod-w3-01`

стали `Running`.

Что было подтверждено внутри pod до benchmark:

- `nvidia-smi -L` показывает `Tesla V100-PCIE-32GB`;
- в `/dev/infiniband` есть `uverbs0`;
- `networks-status` показывает нужный `UnderlayNetwork` и direct netdev:
  - `w1`: `enp193s0np0`
  - `w3`: `enp2s0np0`.

### 13.4. CUDA-aware `perftest` inside pod

Ubuntu package `perftest` внутри `nvidia/cuda:12.2.2-devel-ubuntu22.04` не
содержал `--use_cuda*`, поэтому `perftest` был собран прямо в обоих pod:

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

Подтверждённый результат на обоих pod:

- `--use_cuda`
- `--use_cuda_bus_id`
- `--use_cuda_dmabuf`

### 13.5. Direct pod IP и GID

Для combined pod использовались:

- `gpudirect-pod-w1-c2/enp193s0np0` -> `172.31.130.1/30`
- `gpudirect-pod-w3-01/enp2s0np0` -> `172.31.130.2/30`

После назначения `/30` обе стороны имели IPv4 `RoCE v2 GID index 3`.

GPU, видимые внутри pod:

- `w1`:
  - `GPU 0`
  - `PCI BUS ID 00000000:C2:00.0`
- `w3`:
  - `GPU 0`
  - `PCI BUS ID 00000000:01:00.0`

### 13.6. Functional result

`w3 -> w1`:

```bash
# w1 server
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w1-c2 -- \
  bash -lc 'cd /tmp/perftest && ./ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 5 --use_cuda=0'

# w3 client
kubectl -n sdn-rdma-test exec gpudirect-pod-w3-01 -- \
  bash -lc 'cd /tmp/perftest && ./ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 5 --use_cuda=0 172.31.130.1'
```

Результат:

- `RC=0`
- `BW average 56.42 Gb/sec`
- в client log есть:
  - `initializing CUDA`
  - `device name = [Tesla V100-PCIE-32GB]`
  - `cuMemAlloc() of a 131072 bytes GPU buffer`

`w1 -> w3`:

```bash
# w3 server
kubectl -n sdn-rdma-test exec -i gpudirect-pod-w3-01 -- \
  bash -lc 'cd /tmp/perftest && ./ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 5 --use_cuda=0'

# w1 client
kubectl -n sdn-rdma-test exec gpudirect-pod-w1-c2 -- \
  bash -lc 'cd /tmp/perftest && ./ib_write_bw -d mlx5_0 -x 3 -m 4096 -F --report_gbits -D 5 --use_cuda=0 172.31.130.2'
```

Результат:

- `RC=0`
- `BW average 88.30 Gb/sec`
- в client log опять были `initializing CUDA` и `cuMemAlloc()`

### 13.7. Практический вывод

Подтверждено, что внутри обычного validation pod без `hostPath` и без
`privileged` одновременно работают:

- direct Mellanox PF через `sdn`;
- `/dev/infiniband/uverbs0`;
- GPU device через DRA claim;
- `CUDA init` и выделение GPU buffer;
- межpodовый verbs benchmark поверх GPU memory.

То есть pod-level `GPUDirect RDMA` на текущем стенде уже доказан
функционально.

## 14. Pod-level comparison: `RDMA` vs non-`RDMA` на одном direct path

### 14.1. Цель

После подтверждения pod-level `GPUDirect RDMA` понадобилось собрать отдельный
comparison bundle, который можно показать не только `RDMA`-команде, но и
коллегам, привыкшим читать обычные `TCP` метрики.

Ограничение было жёстким:

- сравнивать надо один и тот же pod-to-pod underlay path;
- не смешивать direct `RDMA` path и overlay/pod IP;
- выбрать метрики, которые можно потом руками повторить в secure contour.

### 14.2. Выбранные canonical metrics

- non-`RDMA` throughput:
  - `iperf3`
  - `Gb/sec`
- non-`RDMA` latency:
  - `sockperf ping-pong`
  - `usec`
- `RDMA` throughput:
  - `ib_write_bw`
  - `Gb/sec`
- `RDMA` latency:
  - `ib_send_lat`
  - `usec`

Это не покрывает `GPU memory` path; для этого остаётся отдельный bundle из
секции 13 с `--use_cuda*`, `initializing CUDA` и `cuMemAlloc()`.

### 14.3. Test setup

Использовались уже существующие validation pod:

- `rdma-pod-w1-c0`
- `rdma-pod-w3-00`

Временный direct `/30`:

- `rdma-pod-w1-c0/enp193s0np0` -> `172.31.141.1/30`
- `rdma-pod-w3-00/enp2s0np0` -> `172.31.141.2/30`

После назначения IP обе стороны использовали IPv4 `RoCE v2 GID index 3`.

### 14.4. Non-RDMA throughput

`w3 -> w1`:

```bash
# server
kubectl -n sdn-rdma-test exec -i rdma-pod-w1-c0 -- \
  bash -lc 'iperf3 -s -B 172.31.141.1 -1'

# client
kubectl -n sdn-rdma-test exec rdma-pod-w3-00 -- \
  bash -lc 'iperf3 -c 172.31.141.1 -B 172.31.141.2 -P 4 -t 10'
```

Результат:

- `sender 14.2 Gb/sec`
- `receiver 14.1 Gb/sec`

`w1 -> w3`:

```bash
kubectl -n sdn-rdma-test exec -i rdma-pod-w1-c0 -- \
  bash -lc 'iperf3 -s -B 172.31.141.1 -1'

kubectl -n sdn-rdma-test exec rdma-pod-w3-00 -- \
  bash -lc 'iperf3 -c 172.31.141.1 -B 172.31.141.2 -P 4 -t 10 -R'
```

Результат:

- `sender 22.2 Gb/sec`
- `receiver 22.3 Gb/sec`

Была ещё попытка сделать fairer TCP baseline через `-P 8 -Z`, но она не дала
улучшения:

- `13.8-13.9 Gb/sec`

То есть текущий socket/TCP path на том же direct канале остаётся далеко ниже
line-rate.

### 14.5. Non-RDMA latency

Latency:

```bash
# server
kubectl -n sdn-rdma-test exec -i rdma-pod-w1-c0 -- \
  bash -lc 'sockperf server --tcp -i 172.31.141.1'

# client
kubectl -n sdn-rdma-test exec rdma-pod-w3-00 -- \
  bash -lc 'sockperf ping-pong --tcp -i 172.31.141.1 --client_ip 172.31.141.2 -m 64 -t 10'
```

Результат:

- `avg-latency 39.181 usec`
- `p50 38.840 usec`
- `p99 53.350 usec`
- `p99.9 120.381 usec`

RTT variant:

```bash
kubectl -n sdn-rdma-test exec -i rdma-pod-w1-c0 -- \
  bash -lc 'sockperf server --tcp -i 172.31.141.1'

kubectl -n sdn-rdma-test exec rdma-pod-w3-00 -- \
  bash -lc 'sockperf ping-pong --tcp -i 172.31.141.1 --client_ip 172.31.141.2 -m 64 -t 10 --full-rtt'
```

Результат:

- `avg-rtt 80.301 usec`
- `p50 79.401 usec`
- `p99 107.282 usec`
- `p99.9 261.194 usec`

### 14.6. RDMA throughput

`w3 -> w1`:

```bash
# server
kubectl -n sdn-rdma-test exec -i rdma-pod-w1-c0 -- \
  bash -lc 'ib_write_bw -d mlx5_0 -x 3 -m 4096 -q 8 -D 10 -F --report_gbits'

# client
kubectl -n sdn-rdma-test exec rdma-pod-w3-00 -- \
  bash -lc 'ib_write_bw -d mlx5_0 -x 3 -m 4096 -q 8 -D 10 -F --report_gbits 172.31.141.1'
```

Результат:

- `BW average 97.24 Gb/sec`

`w1 -> w3`:

```bash
# server
kubectl -n sdn-rdma-test exec -i rdma-pod-w3-00 -- \
  bash -lc 'ib_write_bw -d mlx5_0 -x 3 -m 4096 -q 8 -D 10 -F --report_gbits'

# client
kubectl -n sdn-rdma-test exec rdma-pod-w1-c0 -- \
  bash -lc 'ib_write_bw -d mlx5_0 -x 3 -m 4096 -q 8 -D 10 -F --report_gbits 172.31.141.2'
```

Результат:

- `BW average 97.07 Gb/sec`

### 14.7. RDMA latency

```bash
# server
kubectl -n sdn-rdma-test exec -i rdma-pod-w1-c0 -- \
  bash -lc 'ib_send_lat -d mlx5_0 -x 3 -m 4096 -s 64 -F -n 10000'

# client
kubectl -n sdn-rdma-test exec rdma-pod-w3-00 -- \
  bash -lc 'ib_send_lat -d mlx5_0 -x 3 -m 4096 -s 64 -F -n 10000 172.31.141.1'
```

Результат:

- `t_avg 2.99 usec`
- `99% 5.66 usec`
- `99.9% 11.26 usec`

### 14.8. Итог comparison

На одном и том же direct pod path:

- TCP throughput: `14.1-22.3 Gb/sec`
- TCP latency: `39.18 usec`
- RDMA throughput: `97.07-97.24 Gb/sec`
- RDMA latency: `2.99 usec`

Практическое чтение:

- по throughput `RDMA` здесь быстрее TCP примерно в `4.4-6.9x`;
- по latency `RDMA` здесь быстрее TCP примерно в `13x`;
- разница уже настолько большая, что её можно использовать как operator-facing
  доказательство полезности `RDMA` без сложной `RDMA`-специфичной интерпретации.

### 14.9. Cleanup

После теста временные `/30` были удалены:

```bash
kubectl -n sdn-rdma-test exec rdma-pod-w1-c0 -- \
  bash -lc 'ip addr del 172.31.141.1/30 dev enp193s0np0 || true'

kubectl -n sdn-rdma-test exec rdma-pod-w3-00 -- \
  bash -lc 'ip addr del 172.31.141.2/30 dev enp2s0np0 || true'
```

## 15. Market / upstream references

### Kubernetes upstream

- DRA terminology, `DeviceClass`, `ResourceClaimTemplate`, `ResourceSlice`, CEL
  selectors и per-pod claims:
  <https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/>
- DRA prioritized subrequests и node scoring by higher-ranked alternative:
  <https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/>
- DRA partitionable/logical devices:
  <https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/>
- Scheduler extension guidance: scheduler framework / multiple schedulers /
  extenders only filter+prioritize:
  <https://kubernetes.io/docs/concepts/extend-kubernetes>
  <https://kubernetes.io/docs/tasks/extend-kubernetes/configure-multiple-schedulers/>
- Topology Manager `pod` scope and `single-numa-node` / `restricted` policies:
  <https://kubernetes.io/docs/tasks/administer-cluster/topology-manager/>

### NVIDIA references

- GPU Operator DRA driver and higher-level `ComputeDomain` abstraction:
  <https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/dra-intro-install.html>
  <https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/25.10/dra-cds.html>
- Network Operator as current RDMA market baseline:
  shared RDMA device plugin, SR-IOV, secondary networks, exclusive RDMA mode:
  <https://docs.nvidia.com/networking/display/kubernetes2610/deployment-guide-kubernetes.html>
- GPUDirect RDMA host-side prerequisites and `nvidia-peermem`:
  <https://docs.nvidia.com/cuda/gpudirect-rdma/>
- Dynamo / KAI Scheduler / topology-aware scheduling as reference for
  topology-driven AI placement:
  <https://docs.nvidia.com/dynamo/dev/kubernetes-deployment/multinode/topology-aware-scheduling>

### OpenShift / Red Hat reference

- NUMA Resources Operator / secondary NUMA-aware scheduler / NodeResourceTopology:
  <https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/scalability_and_performance/cnf-numa-aware-scheduling>

## 16. Recommended next bundle

Если продолжать эту тему в `sdn`, следующий bounded bundle должен быть уже не
research, а implementation-oriented:

1. Зафиксировать exact experimental RDMA contract для Mellanox:
   - required device nodes;
   - required sysfs;
   - example pod manifest.
2. Добавить explicit e2e/smoke path:
   - verbs container without GPU;
   - optional GPUDirect validation on prepared hardware.
3. Отдельно решить архитектурно:
   - composite `GPU+NIC` DRA driver;
   - или scheduler framework / secondary scheduler поверх topology inventory.

## 17. Reusable validation toolkit

После ручных live-проверок bundle был доведён до reusable toolkit surface:

- directory:
  `plans/active/research-sdn-underlay-rdma-dra-gpu-placement/toolkit/rdma-validation-toolkit/`
- files:
  - `Containerfile`
  - `bin/rdma-validate`
  - `manifests/rdma-validation-pod.yaml`
  - `manifests/gpudirect-validation-pod.yaml`
  - `README.ru.md`

Что именно добавлено поверх raw команд:

- единый entrypoint для:
  - `inventory`
  - `bind-ip` / `cleanup-ip`
  - `tcp-bw-*`
  - `tcp-lat-*`
  - `tcp-rtt-client`
  - `rdma-bw-*`
  - `rdma-lat-*`
  - `gpudirect-bw-*`
- structured JSONL output в `results.jsonl`;
- `report` для короткой агрегированной сводки;
- `check` для threshold-based regression smoke;
- `GPUDirect` summary теперь отдельно фиксирует:
  - `cuda_used`
  - `gpu_name`
  - `gpu_selector`.

Практический вывод:

- emergency fallback "собрать `perftest` прямо в pod" оставлен только как
  исторический шаг и debugging fallback;
- operator-facing canonical path теперь должен идти через этот toolkit image.
