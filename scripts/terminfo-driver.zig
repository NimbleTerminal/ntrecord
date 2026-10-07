// Prints Ghostty's terminfo source for xterm-ghostty. Run by scripts/terminfo.sh
// against the pinned Ghostty checkout (module "terminfo" = src/terminfo/main.zig).
const std = @import("std");
const terminfo = @import("terminfo");
pub fn main(init: std.process.Init) !void {
    var buffer: [1024]u8 = undefined;
    var stdout_writer = std.Io.File.stdout().writerStreaming(init.io, &buffer);
    const writer = &stdout_writer.interface;
    try terminfo.ghostty.encode(writer);
    try stdout_writer.end();
}
