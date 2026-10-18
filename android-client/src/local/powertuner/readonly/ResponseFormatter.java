package local.powertuner.readonly;

import java.nio.ByteBuffer;
import java.nio.charset.*;

/** Display encodings only. Does not change the request or interpret ECU binary framing. */
public final class ResponseFormatter {
    public enum Mode { TEXT, BINARY, HEX, ALL }
    private static final int PREVIEW_BYTES = 16384;
    private static final char[] HEX = "0123456789abcdef".toCharArray();
    public static String format(byte[] body, Mode mode) {
        int count = Math.min(body.length, PREVIEW_BYTES);
        StringBuilder out = new StringBuilder();
        if (mode == Mode.ALL) out.append("JSON / UTF-8 text\n");
        if (mode == Mode.TEXT || mode == Mode.ALL) {
            try {
                out.append(StandardCharsets.UTF_8.newDecoder().onMalformedInput(CodingErrorAction.REPORT)
                    .onUnmappableCharacter(CodingErrorAction.REPORT).decode(ByteBuffer.wrap(body, 0, count)));
            } catch (CharacterCodingException e) {
                out.append("[Preview is not valid UTF-8; use Hex/Binary or save original bytes.]");
            }
        }
        if (mode == Mode.ALL) out.append("\n\nBINARY (8 bits per byte)\n");
        if (mode == Mode.BINARY || mode == Mode.ALL) dump(body, count, true, out);
        if (mode == Mode.ALL) out.append("\n\nHEX\n");
        if (mode == Mode.HEX || mode == Mode.ALL) dump(body, count, false, out);
        if (count < body.length) out.append("\n[Preview limited to first ").append(count)
            .append(" of ").append(body.length).append(" bytes. Save original bytes for the full response.]");
        return out.toString();
    }
    private static void dump(byte[] bytes, int count, boolean binary, StringBuilder out) {
        int perLine = binary ? 8 : 16;
        for (int i = 0; i < count; i++) {
            if (i % perLine == 0) {
                if (i > 0) out.append('\n');
                String offset = Integer.toHexString(i);
                for (int j = offset.length(); j < 8; j++) out.append('0');
                out.append(offset).append(": ");
            } else out.append(' ');
            int value = bytes[i] & 255;
            if (binary) {
                for (int bit = 7; bit >= 0; bit--) out.append((value >>> bit) & 1);
            } else out.append(HEX[value >>> 4]).append(HEX[value & 15]);
        }
    }
}
