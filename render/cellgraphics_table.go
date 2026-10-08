// Box topology from the Unicode Box Drawing character names (U+2500–U+257F).
package render

var boxDrawings = [...]boxDrawing{
	{[4]uint8{1, 1, 0, 0}, 0, false}, // ─ BOX DRAWINGS LIGHT HORIZONTAL
	{[4]uint8{2, 2, 0, 0}, 0, false}, // ━ BOX DRAWINGS HEAVY HORIZONTAL
	{[4]uint8{0, 0, 1, 1}, 0, false}, // │ BOX DRAWINGS LIGHT VERTICAL
	{[4]uint8{0, 0, 2, 2}, 0, false}, // ┃ BOX DRAWINGS HEAVY VERTICAL
	{[4]uint8{1, 1, 0, 0}, 3, false}, // ┄ BOX DRAWINGS LIGHT TRIPLE DASH HORIZONTAL
	{[4]uint8{2, 2, 0, 0}, 3, false}, // ┅ BOX DRAWINGS HEAVY TRIPLE DASH HORIZONTAL
	{[4]uint8{0, 0, 1, 1}, 3, false}, // ┆ BOX DRAWINGS LIGHT TRIPLE DASH VERTICAL
	{[4]uint8{0, 0, 2, 2}, 3, false}, // ┇ BOX DRAWINGS HEAVY TRIPLE DASH VERTICAL
	{[4]uint8{1, 1, 0, 0}, 4, false}, // ┈ BOX DRAWINGS LIGHT QUADRUPLE DASH HORIZONTAL
	{[4]uint8{2, 2, 0, 0}, 4, false}, // ┉ BOX DRAWINGS HEAVY QUADRUPLE DASH HORIZONTAL
	{[4]uint8{0, 0, 1, 1}, 4, false}, // ┊ BOX DRAWINGS LIGHT QUADRUPLE DASH VERTICAL
	{[4]uint8{0, 0, 2, 2}, 4, false}, // ┋ BOX DRAWINGS HEAVY QUADRUPLE DASH VERTICAL
	{[4]uint8{0, 1, 0, 1}, 0, false}, // ┌ BOX DRAWINGS LIGHT DOWN AND RIGHT
	{[4]uint8{0, 2, 0, 1}, 0, false}, // ┍ BOX DRAWINGS DOWN LIGHT AND RIGHT HEAVY
	{[4]uint8{0, 1, 0, 2}, 0, false}, // ┎ BOX DRAWINGS DOWN HEAVY AND RIGHT LIGHT
	{[4]uint8{0, 2, 0, 2}, 0, false}, // ┏ BOX DRAWINGS HEAVY DOWN AND RIGHT
	{[4]uint8{1, 0, 0, 1}, 0, false}, // ┐ BOX DRAWINGS LIGHT DOWN AND LEFT
	{[4]uint8{2, 0, 0, 1}, 0, false}, // ┑ BOX DRAWINGS DOWN LIGHT AND LEFT HEAVY
	{[4]uint8{1, 0, 0, 2}, 0, false}, // ┒ BOX DRAWINGS DOWN HEAVY AND LEFT LIGHT
	{[4]uint8{2, 0, 0, 2}, 0, false}, // ┓ BOX DRAWINGS HEAVY DOWN AND LEFT
	{[4]uint8{0, 1, 1, 0}, 0, false}, // └ BOX DRAWINGS LIGHT UP AND RIGHT
	{[4]uint8{0, 2, 1, 0}, 0, false}, // ┕ BOX DRAWINGS UP LIGHT AND RIGHT HEAVY
	{[4]uint8{0, 1, 2, 0}, 0, false}, // ┖ BOX DRAWINGS UP HEAVY AND RIGHT LIGHT
	{[4]uint8{0, 2, 2, 0}, 0, false}, // ┗ BOX DRAWINGS HEAVY UP AND RIGHT
	{[4]uint8{1, 0, 1, 0}, 0, false}, // ┘ BOX DRAWINGS LIGHT UP AND LEFT
	{[4]uint8{2, 0, 1, 0}, 0, false}, // ┙ BOX DRAWINGS UP LIGHT AND LEFT HEAVY
	{[4]uint8{1, 0, 2, 0}, 0, false}, // ┚ BOX DRAWINGS UP HEAVY AND LEFT LIGHT
	{[4]uint8{2, 0, 2, 0}, 0, false}, // ┛ BOX DRAWINGS HEAVY UP AND LEFT
	{[4]uint8{0, 1, 1, 1}, 0, false}, // ├ BOX DRAWINGS LIGHT VERTICAL AND RIGHT
	{[4]uint8{0, 2, 1, 1}, 0, false}, // ┝ BOX DRAWINGS VERTICAL LIGHT AND RIGHT HEAVY
	{[4]uint8{0, 1, 2, 1}, 0, false}, // ┞ BOX DRAWINGS UP HEAVY AND RIGHT DOWN LIGHT
	{[4]uint8{0, 1, 1, 2}, 0, false}, // ┟ BOX DRAWINGS DOWN HEAVY AND RIGHT UP LIGHT
	{[4]uint8{0, 1, 2, 2}, 0, false}, // ┠ BOX DRAWINGS VERTICAL HEAVY AND RIGHT LIGHT
	{[4]uint8{0, 2, 2, 1}, 0, false}, // ┡ BOX DRAWINGS DOWN LIGHT AND RIGHT UP HEAVY
	{[4]uint8{0, 2, 1, 2}, 0, false}, // ┢ BOX DRAWINGS UP LIGHT AND RIGHT DOWN HEAVY
	{[4]uint8{0, 2, 2, 2}, 0, false}, // ┣ BOX DRAWINGS HEAVY VERTICAL AND RIGHT
	{[4]uint8{1, 0, 1, 1}, 0, false}, // ┤ BOX DRAWINGS LIGHT VERTICAL AND LEFT
	{[4]uint8{2, 0, 1, 1}, 0, false}, // ┥ BOX DRAWINGS VERTICAL LIGHT AND LEFT HEAVY
	{[4]uint8{1, 0, 2, 1}, 0, false}, // ┦ BOX DRAWINGS UP HEAVY AND LEFT DOWN LIGHT
	{[4]uint8{1, 0, 1, 2}, 0, false}, // ┧ BOX DRAWINGS DOWN HEAVY AND LEFT UP LIGHT
	{[4]uint8{1, 0, 2, 2}, 0, false}, // ┨ BOX DRAWINGS VERTICAL HEAVY AND LEFT LIGHT
	{[4]uint8{2, 0, 2, 1}, 0, false}, // ┩ BOX DRAWINGS DOWN LIGHT AND LEFT UP HEAVY
	{[4]uint8{2, 0, 1, 2}, 0, false}, // ┪ BOX DRAWINGS UP LIGHT AND LEFT DOWN HEAVY
	{[4]uint8{2, 0, 2, 2}, 0, false}, // ┫ BOX DRAWINGS HEAVY VERTICAL AND LEFT
	{[4]uint8{1, 1, 0, 1}, 0, false}, // ┬ BOX DRAWINGS LIGHT DOWN AND HORIZONTAL
	{[4]uint8{2, 1, 0, 1}, 0, false}, // ┭ BOX DRAWINGS LEFT HEAVY AND RIGHT DOWN LIGHT
	{[4]uint8{1, 2, 0, 1}, 0, false}, // ┮ BOX DRAWINGS RIGHT HEAVY AND LEFT DOWN LIGHT
	{[4]uint8{2, 2, 0, 1}, 0, false}, // ┯ BOX DRAWINGS DOWN LIGHT AND HORIZONTAL HEAVY
	{[4]uint8{1, 1, 0, 2}, 0, false}, // ┰ BOX DRAWINGS DOWN HEAVY AND HORIZONTAL LIGHT
	{[4]uint8{2, 1, 0, 2}, 0, false}, // ┱ BOX DRAWINGS RIGHT LIGHT AND LEFT DOWN HEAVY
	{[4]uint8{1, 2, 0, 2}, 0, false}, // ┲ BOX DRAWINGS LEFT LIGHT AND RIGHT DOWN HEAVY
	{[4]uint8{2, 2, 0, 2}, 0, false}, // ┳ BOX DRAWINGS HEAVY DOWN AND HORIZONTAL
	{[4]uint8{1, 1, 1, 0}, 0, false}, // ┴ BOX DRAWINGS LIGHT UP AND HORIZONTAL
	{[4]uint8{2, 1, 1, 0}, 0, false}, // ┵ BOX DRAWINGS LEFT HEAVY AND RIGHT UP LIGHT
	{[4]uint8{1, 2, 1, 0}, 0, false}, // ┶ BOX DRAWINGS RIGHT HEAVY AND LEFT UP LIGHT
	{[4]uint8{2, 2, 1, 0}, 0, false}, // ┷ BOX DRAWINGS UP LIGHT AND HORIZONTAL HEAVY
	{[4]uint8{1, 1, 2, 0}, 0, false}, // ┸ BOX DRAWINGS UP HEAVY AND HORIZONTAL LIGHT
	{[4]uint8{2, 1, 2, 0}, 0, false}, // ┹ BOX DRAWINGS RIGHT LIGHT AND LEFT UP HEAVY
	{[4]uint8{1, 2, 2, 0}, 0, false}, // ┺ BOX DRAWINGS LEFT LIGHT AND RIGHT UP HEAVY
	{[4]uint8{2, 2, 2, 0}, 0, false}, // ┻ BOX DRAWINGS HEAVY UP AND HORIZONTAL
	{[4]uint8{1, 1, 1, 1}, 0, false}, // ┼ BOX DRAWINGS LIGHT VERTICAL AND HORIZONTAL
	{[4]uint8{2, 1, 1, 1}, 0, false}, // ┽ BOX DRAWINGS LEFT HEAVY AND RIGHT VERTICAL LIGHT
	{[4]uint8{1, 2, 1, 1}, 0, false}, // ┾ BOX DRAWINGS RIGHT HEAVY AND LEFT VERTICAL LIGHT
	{[4]uint8{2, 2, 1, 1}, 0, false}, // ┿ BOX DRAWINGS VERTICAL LIGHT AND HORIZONTAL HEAVY
	{[4]uint8{1, 1, 2, 1}, 0, false}, // ╀ BOX DRAWINGS UP HEAVY AND DOWN HORIZONTAL LIGHT
	{[4]uint8{1, 1, 1, 2}, 0, false}, // ╁ BOX DRAWINGS DOWN HEAVY AND UP HORIZONTAL LIGHT
	{[4]uint8{1, 1, 2, 2}, 0, false}, // ╂ BOX DRAWINGS VERTICAL HEAVY AND HORIZONTAL LIGHT
	{[4]uint8{2, 1, 2, 1}, 0, false}, // ╃ BOX DRAWINGS LEFT UP HEAVY AND RIGHT DOWN LIGHT
	{[4]uint8{1, 2, 2, 1}, 0, false}, // ╄ BOX DRAWINGS RIGHT UP HEAVY AND LEFT DOWN LIGHT
	{[4]uint8{2, 1, 1, 2}, 0, false}, // ╅ BOX DRAWINGS LEFT DOWN HEAVY AND RIGHT UP LIGHT
	{[4]uint8{1, 2, 1, 2}, 0, false}, // ╆ BOX DRAWINGS RIGHT DOWN HEAVY AND LEFT UP LIGHT
	{[4]uint8{2, 2, 2, 1}, 0, false}, // ╇ BOX DRAWINGS DOWN LIGHT AND UP HORIZONTAL HEAVY
	{[4]uint8{2, 2, 1, 2}, 0, false}, // ╈ BOX DRAWINGS UP LIGHT AND DOWN HORIZONTAL HEAVY
	{[4]uint8{2, 1, 2, 2}, 0, false}, // ╉ BOX DRAWINGS RIGHT LIGHT AND LEFT VERTICAL HEAVY
	{[4]uint8{1, 2, 2, 2}, 0, false}, // ╊ BOX DRAWINGS LEFT LIGHT AND RIGHT VERTICAL HEAVY
	{[4]uint8{2, 2, 2, 2}, 0, false}, // ╋ BOX DRAWINGS HEAVY VERTICAL AND HORIZONTAL
	{[4]uint8{1, 1, 0, 0}, 2, false}, // ╌ BOX DRAWINGS LIGHT DOUBLE DASH HORIZONTAL
	{[4]uint8{2, 2, 0, 0}, 2, false}, // ╍ BOX DRAWINGS HEAVY DOUBLE DASH HORIZONTAL
	{[4]uint8{0, 0, 1, 1}, 2, false}, // ╎ BOX DRAWINGS LIGHT DOUBLE DASH VERTICAL
	{[4]uint8{0, 0, 2, 2}, 2, false}, // ╏ BOX DRAWINGS HEAVY DOUBLE DASH VERTICAL
	{[4]uint8{3, 3, 0, 0}, 0, false}, // ═ BOX DRAWINGS DOUBLE HORIZONTAL
	{[4]uint8{0, 0, 3, 3}, 0, false}, // ║ BOX DRAWINGS DOUBLE VERTICAL
	{[4]uint8{0, 3, 0, 1}, 0, false}, // ╒ BOX DRAWINGS DOWN SINGLE AND RIGHT DOUBLE
	{[4]uint8{0, 1, 0, 3}, 0, false}, // ╓ BOX DRAWINGS DOWN DOUBLE AND RIGHT SINGLE
	{[4]uint8{0, 3, 0, 3}, 0, false}, // ╔ BOX DRAWINGS DOUBLE DOWN AND RIGHT
	{[4]uint8{3, 0, 0, 1}, 0, false}, // ╕ BOX DRAWINGS DOWN SINGLE AND LEFT DOUBLE
	{[4]uint8{1, 0, 0, 3}, 0, false}, // ╖ BOX DRAWINGS DOWN DOUBLE AND LEFT SINGLE
	{[4]uint8{3, 0, 0, 3}, 0, false}, // ╗ BOX DRAWINGS DOUBLE DOWN AND LEFT
	{[4]uint8{0, 3, 1, 0}, 0, false}, // ╘ BOX DRAWINGS UP SINGLE AND RIGHT DOUBLE
	{[4]uint8{0, 1, 3, 0}, 0, false}, // ╙ BOX DRAWINGS UP DOUBLE AND RIGHT SINGLE
	{[4]uint8{0, 3, 3, 0}, 0, false}, // ╚ BOX DRAWINGS DOUBLE UP AND RIGHT
	{[4]uint8{3, 0, 1, 0}, 0, false}, // ╛ BOX DRAWINGS UP SINGLE AND LEFT DOUBLE
	{[4]uint8{1, 0, 3, 0}, 0, false}, // ╜ BOX DRAWINGS UP DOUBLE AND LEFT SINGLE
	{[4]uint8{3, 0, 3, 0}, 0, false}, // ╝ BOX DRAWINGS DOUBLE UP AND LEFT
	{[4]uint8{0, 3, 1, 1}, 0, false}, // ╞ BOX DRAWINGS VERTICAL SINGLE AND RIGHT DOUBLE
	{[4]uint8{0, 1, 3, 3}, 0, false}, // ╟ BOX DRAWINGS VERTICAL DOUBLE AND RIGHT SINGLE
	{[4]uint8{0, 3, 3, 3}, 0, false}, // ╠ BOX DRAWINGS DOUBLE VERTICAL AND RIGHT
	{[4]uint8{3, 0, 1, 1}, 0, false}, // ╡ BOX DRAWINGS VERTICAL SINGLE AND LEFT DOUBLE
	{[4]uint8{1, 0, 3, 3}, 0, false}, // ╢ BOX DRAWINGS VERTICAL DOUBLE AND LEFT SINGLE
	{[4]uint8{3, 0, 3, 3}, 0, false}, // ╣ BOX DRAWINGS DOUBLE VERTICAL AND LEFT
	{[4]uint8{3, 3, 0, 1}, 0, false}, // ╤ BOX DRAWINGS DOWN SINGLE AND HORIZONTAL DOUBLE
	{[4]uint8{1, 1, 0, 3}, 0, false}, // ╥ BOX DRAWINGS DOWN DOUBLE AND HORIZONTAL SINGLE
	{[4]uint8{3, 3, 0, 3}, 0, false}, // ╦ BOX DRAWINGS DOUBLE DOWN AND HORIZONTAL
	{[4]uint8{3, 3, 1, 0}, 0, false}, // ╧ BOX DRAWINGS UP SINGLE AND HORIZONTAL DOUBLE
	{[4]uint8{1, 1, 3, 0}, 0, false}, // ╨ BOX DRAWINGS UP DOUBLE AND HORIZONTAL SINGLE
	{[4]uint8{3, 3, 3, 0}, 0, false}, // ╩ BOX DRAWINGS DOUBLE UP AND HORIZONTAL
	{[4]uint8{3, 3, 1, 1}, 0, false}, // ╪ BOX DRAWINGS VERTICAL SINGLE AND HORIZONTAL DOUBLE
	{[4]uint8{1, 1, 3, 3}, 0, false}, // ╫ BOX DRAWINGS VERTICAL DOUBLE AND HORIZONTAL SINGLE
	{[4]uint8{3, 3, 3, 3}, 0, false}, // ╬ BOX DRAWINGS DOUBLE VERTICAL AND HORIZONTAL
	{[4]uint8{0, 1, 0, 1}, 0, true},  // ╭ BOX DRAWINGS LIGHT ARC DOWN AND RIGHT
	{[4]uint8{1, 0, 0, 1}, 0, true},  // ╮ BOX DRAWINGS LIGHT ARC DOWN AND LEFT
	{[4]uint8{1, 0, 1, 0}, 0, true},  // ╯ BOX DRAWINGS LIGHT ARC UP AND LEFT
	{[4]uint8{0, 1, 1, 0}, 0, true},  // ╰ BOX DRAWINGS LIGHT ARC UP AND RIGHT
	{[4]uint8{0, 0, 0, 0}, 0, false}, // ╱ BOX DRAWINGS LIGHT DIAGONAL UPPER RIGHT TO LOWER LEFT
	{[4]uint8{0, 0, 0, 0}, 0, false}, // ╲ BOX DRAWINGS LIGHT DIAGONAL UPPER LEFT TO LOWER RIGHT
	{[4]uint8{0, 0, 0, 0}, 0, false}, // ╳ BOX DRAWINGS LIGHT DIAGONAL CROSS
	{[4]uint8{1, 0, 0, 0}, 0, false}, // ╴ BOX DRAWINGS LIGHT LEFT
	{[4]uint8{0, 0, 1, 0}, 0, false}, // ╵ BOX DRAWINGS LIGHT UP
	{[4]uint8{0, 1, 0, 0}, 0, false}, // ╶ BOX DRAWINGS LIGHT RIGHT
	{[4]uint8{0, 0, 0, 1}, 0, false}, // ╷ BOX DRAWINGS LIGHT DOWN
	{[4]uint8{2, 0, 0, 0}, 0, false}, // ╸ BOX DRAWINGS HEAVY LEFT
	{[4]uint8{0, 0, 2, 0}, 0, false}, // ╹ BOX DRAWINGS HEAVY UP
	{[4]uint8{0, 2, 0, 0}, 0, false}, // ╺ BOX DRAWINGS HEAVY RIGHT
	{[4]uint8{0, 0, 0, 2}, 0, false}, // ╻ BOX DRAWINGS HEAVY DOWN
	{[4]uint8{1, 2, 0, 0}, 0, false}, // ╼ BOX DRAWINGS LIGHT LEFT AND HEAVY RIGHT
	{[4]uint8{0, 0, 1, 2}, 0, false}, // ╽ BOX DRAWINGS LIGHT UP AND HEAVY DOWN
	{[4]uint8{2, 1, 0, 0}, 0, false}, // ╾ BOX DRAWINGS HEAVY LEFT AND LIGHT RIGHT
	{[4]uint8{0, 0, 2, 1}, 0, false}, // ╿ BOX DRAWINGS HEAVY UP AND LIGHT DOWN
}
