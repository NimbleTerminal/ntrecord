# GIF encoding

GIF output uses one adaptive palette for the whole recording. Theme background,
foreground, cursor, ANSI colors, margin fill, window-bar colors, black and white
remain exact. Up to 32 other frequent flat colors are reserved too: an application
or Kitty image can introduce a solid background absent from the terminal theme.
Remaining entries use weighted median cut over a bounded RGB histogram.

The palette never changes between frames. Floyd–Steinberg dithering still handles
image gradients and antialiased edges. An exact source color is restored to its
exact palette index after dithering, so neighboring error cannot speckle a flat
background or QR module. Palette quantization is **not lossless**. Fine gradients
can still show stippling or bands; the palette is shared among every scene, so a
long, visually diverse recording may favor some colors over others.

The histogram has 32,768 RGB bins and samples at most 65,536 pixels per distinct
source frame. Horizontal runs of at least eight identical pixels nominate exact
flat colors using a bounded weighted-majority candidate in each bin. At most 32
candidates survive, ordered deterministically by frequency and RGB. Seeds take
priority. Median-cut splitting and tie breaks use integers for reproducibility.
Flat-color candidates are a heuristic, not a promise to preserve every color.

## Patches, timing and loops

Encoding keeps the first frame as an opaque full canvas, then writes the bounding
rectangle of pixels that changed. If useful, unchanged pixels inside that rectangle
become transparent. Their index is borrowed from a color unused by the changed
pixels, or appended if the table has room; a patch needing every index stays opaque.
Actual LZW sizes decide between opaque and transparent patches, including a color
table expansion when necessary. Unused global colors are removed losslessly to
shorten LZW codes. No runtime optimizer or external encoder is invoked.

Every frame explicitly uses `DisposalNone`: keep the canvas and overwrite changed
pixels, including pixels where content disappeared. Input RGBA is premultiplied;
transparent source pixels flatten over black. Output transparency only means
“retain an unchanged canvas pixel,” never “erase this pixel.” The first full frame
resets the canvas at each loop boundary. Delta construction happens **after**
`LoopOffset` rotation, including splits inside held frames.

Cumulative centisecond rounding, infinite looping, adjacent identical-source
merging and 65,535-centisecond delay splitting are preserved. Overflow holds may
use a one-pixel patch. Palette construction does not change the timeline.

## Resource accounting and dependencies

Distinct source frames are held as `compress/zlib` BestSpeed data until finalization.
This permits a palette built from later scenes without retaining all full RGBA
frames. Finalization quantizes each distinct source once; split holds share pixels.
No source spool files need cleanup. Capture and rendering behavior are unchanged.

The default 512 MiB conservative working-set budget charges:

- eight MiB for histogram, median-cut, palette maps, zlib and GIF/LZW workspace;
- ten bytes per viewport pixel for source/previous/quantization scratch, plus row scratch;
- two full indexed frame sets, covering original canvases and final patches;
- three times compressed-source length for buffer capacity and growth;
- eight KiB per timeline entry for palettes, descriptors, maps and slice growth.

Checks run before frame allocation and during compressed-buffer writes. Large held
frames also consume timeline metadata; repeated pixels do not bypass the limit.
Rejected additions do not advance the timeline. Rotation and encoding check the
budget too. `Encoder.MaxBytes` can set a smaller budget for embedded callers/tests.
This is not an operating-system RSS limit: Go GC, the renderer, caller-owned input,
and the benchmark's decoder have their own memory. Noisy/high-color recordings
can hit the limit earlier than in the previous encoder. Histogram size and samples,
frame storage, LZW alternatives and palette size are all bounded.

The implementation adds **no dependency**: quantization, zlib and LZW are written
with Go's standard library. No third-party library/license or maintenance burden
was introduced. Gifsicle and FFmpeg are development validation tools only.

## Measurements (2026-10-07)

Local macOS 27.0.1 arm64, Go 1.27.1, FFmpeg 9.0.2, gifsicle 1.96. These are
single-run observations, not performance assertions. Encoding time is summed
inside `Add` and `Encode`, including source compression, histogram/palette building,
dithering, delta selection and output writing; input decoding is excluded. Peak
RSS comes from `/usr/bin/time -l` and includes the complete benchmark process.

The baseline encoder is commit `8c94b8e`. The gloss tape was copied to a temporary
workspace with read-only symlinks to the gloss executable and examples. Output and
Screenshot destinations were redirected there; gloss was not modified. One run
produced 418 PNG frames at 944×664, replayed at 12 FPS (34.83 seconds) for identical
before/after source pixels and timing. This includes terminal text, images, GIF
playback, SVG, PDF, rotating 3D, Markdown, tables and QR overlay. A frame-directory
capture can sample more animation frames than an old live GIF recording, so the
fixed-source corpus and the supplied existing GIF are separate comparisons.
The source tape SHA-256 was
`91eb2eea4d5e37c1716eca524d4e42aaf9bc4b8e60028bec4bc4c4c918bc09fc`;
the supplied GIF SHA-256 was
`f0937434d0e175f1dff4b70f8558168bea36f72d8dea962cb4726d4b60f706d3`.

| Fixed gloss source | GIF bytes | Encoding seconds | Peak RSS MiB |
| --- | ---: | ---: | ---: |
| Old seeded Plan9 encoder | 5,958,350 | 9.128 | 140.89 |
| Changed regions + exact colors, seeded Plan9 | 4,155,528 | 10.122 | 126.11 |
| Final recording-wide adaptive palette + patches | 1,227,456 | 6.675 | 131.92 |
| Old output, then gifsicle `-O3` | 3,507,680 | +1.05 postprocess | 68.64 optimizer only |
| Final output, then gifsicle `-O3` | 987,764 | +0.58 postprocess | 63.95 optimizer only |

Gifsicle rows measure the separate optimizer process (wall seconds and its own
peak RSS); add that time to encoder time for a two-stage pipeline. On the supplied
original gloss GIF, `-O3` took 0.74 seconds with 53.89 MiB peak RSS.

The final output is 79.4% smaller than the original encoder on this corpus, with
26.9% less encoding time and 6.4% lower peak RSS. This size improvement includes
**different quantization**, not just lossless delta encoding. FFmpeg RGB PSNR against
the source PNG timeline rises from 26.69 dB to 46.25 dB. A fixed first-frame adaptive
palette was also tried and rejected: 5,506,958 bytes, 25.93 dB. The initial terminal
frame did not contain representative image colors for later scenes.

Existing GIF fixtures were independently replayed with their original palette
seeded, isolating lossless delta/table changes rather than trying to recover
colors already lost by old quantization. Every row below has identical FFmpeg
RGBA frame hashes **and timestamps** before/after.

| Existing fixture | Old bytes | New bytes | Old + `-O3` bytes | Encode seconds old → new | Peak RSS MiB old → new |
| --- | ---: | ---: | ---: | ---: | ---: |
| Supplied gloss showcase (66 frames) | 4,569,113 | 2,962,016 | 2,507,307 | 1.691 → 2.673 | 149.70 → 135.98 |
| `examples/text.gif` | 425,835 | 44,024 | 20,423 | 0.502 → 0.939 | 79.55 → 62.67 |
| `examples/kitty.gif` | 16,643 | 14,507 | 14,100 | 0.013 → 0.025 | 10.84 → 14.83 |
| `examples/language.gif` | 490,911 | 43,577 | 12,928 | 0.539 → 0.724 | 66.92 → 52.28 |
| `examples/shaders.gif` | 9,014,729 | 8,830,415 | 8,683,486 | 3.692 → 4.806 | 88.41 → 120.12 |

Gifsicle still compresses better; this encoder does not reproduce its broader
optimization search. Dense shader changes offer little spatial redundancy and
pay extra source-retention/compression cost. Small clips also pay fixed workspace
cost. No lossy gifsicle mode was used or added.

The final CLI also completed the full isolated gloss tape: 1345413 bytes,
48.57 seconds wall time including the tape's sleeps, 12.56 seconds user CPU and
378.36 MiB process peak RSS including rendering/native libraries. Live capture
timing differs from the fixed PNG corpus; this is an end-to-end smoke check, not
an additional apples-to-apples encoder benchmark.

## Visual and independent checks

Inspected decoded frames at full resolution for text and QR, plus a contact sheet
covering images, GIF motion, SVG, PDF, 3D, Markdown and scene transitions. Solid
backgrounds are substantially cleaner. Landscape gradients retain mild GIF
stipple/banding; they are not identical to the source. Text and thin borders remain
legible, and scene removal/replacement leaves no visible trailing rectangles.

In source frame 346, the 70×300 purple overlay region at (820,100) has one RGB color;
the old GIF has eight dither colors there; the new GIF has one. The 330×342 QR region
at (472,81), including white quiet space and black modules, is **pixel-identical**
to the source. This checks pixel fidelity, not scanning with a QR reader.

`internal/gifencode/delta_test.go` uses FFmpeg as an independent compositor,
comparing optimized GIFs with full-canvas references across two loops and several
LoopOffsets. It covers disappearance, replacement, transparency, opaque fallback
when every palette entry is needed, exact backgrounds beside gradients, late
application colors, deterministic palettes, source compression limits, huge hold
metadata, strided inputs and cumulative timing. Existing delay-overflow and theme
seed tests remain. CI sets `NTRECORD_REQUIRE_FFMPEG=1`, so composition checks cannot
silently skip when FFmpeg is missing.

## Reproduce

Build the benchmark tool, then replay an existing fixture (its first palette is
used as the seed by default):

```sh
go build -o /tmp/gif-bench ./internal/tools/gif-bench
/usr/bin/time -l /tmp/gif-bench -input examples/text.gif -output /tmp/text-new.gif
/usr/bin/time -l gifsicle -O3 /tmp/text-new.gif -o /tmp/text-O3.gif
```

For new source pixels, copy a tape to an isolated workspace, replace its GIF
`Output` with an empty frame directory ending in `/`, redirect every other output
and screenshot, and run the recorder there. Preserve the resulting PNG sequence
for both encoders. Run each measurement sequentially to avoid CPU contention:

```sh
seed=1e1e2e,cdd6f4,f5e0dc,45475a,f38ba8,a6e3a1,f9e2af,89b4fa,f5c2e7,94e2d5,bac2de,585b70,f38ba8,a6e3a1,f9e2af,89b4fa,f5c2e7,94e2d5,a6adc8,201936,ff5f56,ffbd2e,27c93f
/usr/bin/time -l /tmp/gif-bench -input /tmp/gloss-frames -fps 12 -seed "$seed" -output /tmp/adaptive.gif
/usr/bin/time -l /tmp/gif-bench -input /tmp/gloss-frames -fps 12 -seed "$seed" -fixed -output /tmp/fixed.gif
```

To compare the old encoder, build the benchmark driver against `8c94b8e` in an
isolated checkout, removing only the newer `FixedPalette` initializer/flag from
the driver. Do not swap source files in an active recording process. The benchmark
reports timing and sampled heap data as JSON; `/usr/bin/time -l` provides peak RSS
(on Linux, use `/usr/bin/time -v`). GIF input decoding is a development helper,
not the independent correctness oracle; use FFmpeg for that:

```sh
ffmpeg -v error -ignore_loop 1 -min_delay 0 -i /tmp/before.gif -fps_mode passthrough -pix_fmt rgba -f framemd5 /tmp/before.md5
ffmpeg -v error -ignore_loop 1 -min_delay 0 -i /tmp/after.gif -fps_mode passthrough -pix_fmt rgba -f framemd5 /tmp/after.md5
diff /tmp/before.md5 /tmp/after.md5
NTRECORD_REQUIRE_FFMPEG=1 task ci
```

Hash comparison applies to palette-preserving fixtures/delta references. Compare
adaptive output against original RGBA/PNG sources visually and with image-error
metrics instead of asserting losslessness.
