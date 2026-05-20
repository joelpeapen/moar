/*
Package reader provides a unified and advanced reader interface for handling
text from both files and streams.

It is designed to abstract away the complexities of ingestion, stream
processing, and initial text decoration. The primary features of this package
include:

 1. Input Unification: Provides a clean `Reader` interface to query lines
    (`GetLines`, `GetLine`), regardless of whether the source is a static file
    or a continuous stream.
 2. Transparent Decompression: Automatically detects and decompresses formats
    like gzip, bzip2, zstd, and xz on-the-fly. This is handled by the ZOpen
    function, which inspects magic bytes and strips compression extensions.
 3. Content Format Detection and Reformatting: Checks the incoming text for
    valid JSON or XML if highlighting information isn't provided, and optionally
    pretty-prints JSON when requested via ReaderOptions.ShouldFormat.
 4. Live Tailing: Continues to monitor seekable file sources (using `tailFile`)
    for appended bytes or truncated reloads, updating the viewer automatically.
 5. Syntax Highlighting: Applies highlighting using Chroma (alecthomas/chroma).
    Reading and highlighting happen asynchronously in the background so the UI
    doesn't block.
 6. Pausing and Resource Management: Prevents infinite memory consumption by
    intelligently pausing reads at a specific line limit (e.g. 50,000 lines)
    until the user scrolls further down.
 7. Status and Metadata Generation: Generates view-ready status lines indicating
    buffer name, current view position, and trailing progress percentages as the
    data dictates (`createStatusUnlocked`).

By encapsulating these capabilities, the reader package allows the rest of the
application (such as the pager) to consume rich, formatted, lines of text
without worrying about the underlying complexities of text ingestion.
*/
package reader
