package local.powertuner.readonly;

import java.io.*;
import java.net.*;
import java.nio.charset.StandardCharsets;
import java.util.*;

/** Offline JVM tests. No Android app, HTTP server or device is started. */
public final class ReadOnlyClientTest {
    static final class Fake extends HttpURLConnection {
        int status = 200;
        byte[] body = "{\"ok\":true}".getBytes(StandardCharsets.UTF_8);
        boolean disconnected;
        Fake(URL url) { super(url); }
        public void connect() { }
        public boolean usingProxy() { return false; }
        public void disconnect() { disconnected = true; }
        public int getResponseCode() { return status; }
        public InputStream getInputStream() { return new ByteArrayInputStream(body); }
        public OutputStream getOutputStream() { throw new AssertionError("Write stream requested"); }
    }
    static void check(boolean value, String message) { if (!value) throw new AssertionError(message); }
    public static void main(String[] args) throws Exception {
        List<Fake> calls = new ArrayList<>();
        ReadOnlyClient client = new ReadOnlyClient("http://127.0.0.1:8080", url -> {
            Fake fake = new Fake(url); calls.add(fake); return fake;
        });
        String[] expected = {"/info/", "/ecu/settings/", "/eps/settings/", "/ecu/monitor/", "/ecu/model/", "/ecu/lock/", "/ecu/lap/",
            "/ecu/log/?request=0", "/ecu/log/?request=1", "/ecu/log/?request=2", "/ecu/log/?request=3"};
        int index = 0;
        for (ReadOnlyClient.Resource resource : ReadOnlyClient.Resource.values()) {
            check(client.read(resource).equals("{\"ok\":true}"), "Body changed");
            Fake fake = calls.get(index);
            check(fake.getURL().getFile().equals(expected[index++]), "Wrong endpoint");
            check(fake.getRequestMethod().equals("GET"), "Non-GET method");
            check(!fake.getDoOutput(), "Output enabled");
            check(!fake.getInstanceFollowRedirects(), "Redirects enabled");
            check(fake.getConnectTimeout() == 5000 && fake.getReadTimeout() == 5000, "Missing timeout");
            check(fake.disconnected, "Connection not closed");
        }
        for (int status : new int[]{302, 403, 503}) {
            Fake fake = new Fake(new URL("http://localhost/info/")); fake.status = status;
            ReadOnlyClient failure = new ReadOnlyClient("http://localhost", url -> fake);
            try { failure.read(ReadOnlyClient.Resource.INFO); throw new AssertionError("Accepted HTTP " + status); }
            catch (IOException expectedError) { check(expectedError.getMessage().contains("HTTP " + status), "Lost status"); }
            check(fake.disconnected, "Failure connection left open");
        }
        for (String url : new String[]{"file:///tmp/x", "http://host/path", "http://user:pass@host", "http://host?x=1", "http://host/#x"}) {
            try { new ReadOnlyClient(url, ReadOnlyClient::direct); throw new AssertionError("Accepted invalid base URL"); }
            catch (IOException expectedError) { }
        }
        Fake oversized = new Fake(new URL("http://localhost/info/")); oversized.body = new byte[4 * 1024 * 1024 + 1];
        try { new ReadOnlyClient("http://localhost", url -> oversized).read(ReadOnlyClient.Resource.INFO); throw new AssertionError("Accepted large response"); }
        catch (IOException expectedError) { check(expectedError.getMessage().contains("4 MiB"), "Wrong size error"); }
        Thread.currentThread().interrupt();
        try { client.read(ReadOnlyClient.Resource.INFO); throw new AssertionError("Ignored cancellation"); }
        catch (IOException expectedError) { } finally { Thread.interrupted(); }
        check(calls.size() == expected.length, "Unexpected network attempt");
        Fake binary = new Fake(new URL("http://localhost/info/"));
        byte[] allBytes = new byte[256];
        for (int b = 0; b < 256; b++) allBytes[b] = (byte) b;
        binary.body = allBytes;
        byte[] received = new ReadOnlyClient("http://localhost", url -> binary).readBytes(ReadOnlyClient.Resource.INFO);
        check(Arrays.equals(allBytes, received), "Binary response corrupted by text decoding");
        byte[] sample = {0, 1, 15, 16, 127, (byte) 128, (byte) 255};
        String hex = ResponseFormatter.format(sample, ResponseFormatter.Mode.HEX);
        check(hex.equals("00000000: 00 01 0f 10 7f 80 ff"), "Incorrect hex: " + hex);
        String bits = ResponseFormatter.format(sample, ResponseFormatter.Mode.BINARY);
        check(bits.equals("00000000: 00000000 00000001 00001111 00010000 01111111 10000000 11111111"), "Incorrect bits");
        String json = "{\"value\":9007199254740993,\"x\":1.00}";
        check(ResponseFormatter.format(json.getBytes(StandardCharsets.UTF_8), ResponseFormatter.Mode.TEXT).equals(json), "JSON numeric text changed");
        check(ResponseFormatter.format(sample, ResponseFormatter.Mode.TEXT).contains("not valid UTF-8"), "Invalid UTF-8 silently decoded");
        String combined = ResponseFormatter.format(sample, ResponseFormatter.Mode.ALL);
        check(combined.contains(hex) && combined.contains(bits), "Combined display missing views");
        check(ResponseFormatter.format(new byte[16385], ResponseFormatter.Mode.HEX).contains("first 16384 of 16385"), "Large preview unbounded");
        check(ResponseFormatter.format(new byte[0], ResponseFormatter.Mode.HEX).isEmpty(), "Empty body changed");
        System.out.println("PASS: all 11 endpoints GET-only; no write stream; redirects, HTTP errors, invalid URLs, size limit and cancellation checked offline.");
        System.out.println("PASS: all 256 byte values preserved; exact hex/bits; JSON numeric text, invalid UTF-8, combined mode and preview limit checked.");
    }
}
