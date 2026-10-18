package local.powertuner.readonly;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.Proxy;
import java.net.URL;
import java.nio.charset.StandardCharsets;

/** Fixed GET endpoints only; no authentication, upload or arbitrary-path API. */
public final class ReadOnlyClient {
    public enum Resource {
        INFO("/info/"), MAPS("/ecu/settings/"), EPS("/eps/settings/"),
        MONITOR("/ecu/monitor/"), MODEL("/ecu/model/"), LOCK("/ecu/lock/"),
        LAPS("/ecu/lap/"), LOG0("/ecu/log/?request=0"), LOG1("/ecu/log/?request=1"),
        LOG2("/ecu/log/?request=2"), LOG3("/ecu/log/?request=3");
        private final String path;
        Resource(String path) { this.path = path; }
    }
    public interface ConnectionFactory { HttpURLConnection open(URL url) throws IOException; }
    private static final int MAX_BYTES = 4 * 1024 * 1024;
    private final URL base;
    private final ConnectionFactory connections;
    public ReadOnlyClient(String address, ConnectionFactory connections) throws IOException {
        base = new URL(address);
        if (!(base.getProtocol().equals("http") || base.getProtocol().equals("https")) ||
            base.getHost().isEmpty() || base.getUserInfo() != null ||
            !(base.getPath().isEmpty() || base.getPath().equals("/")) ||
            base.getQuery() != null || base.getRef() != null) {
            throw new IOException("Use http(s)://host:port without a path, credentials, query or fragment");
        }
        this.connections = connections;
    }
    public static HttpURLConnection direct(URL url) throws IOException {
        return (HttpURLConnection) url.openConnection(Proxy.NO_PROXY);
    }
    public String read(Resource resource) throws IOException {
        return new String(readBytes(resource), StandardCharsets.UTF_8);
    }
    /** Original HTTP response-body bytes, before any text decoding. */
    public byte[] readBytes(Resource resource) throws IOException {
        if (resource == null) throw new IOException("Select a read endpoint");
        if (Thread.currentThread().isInterrupted()) throw new IOException("Cancelled");
        HttpURLConnection connection = connections.open(new URL(base, resource.path));
        try {
            connection.setRequestMethod("GET");
            connection.setDoOutput(false);
            connection.setInstanceFollowRedirects(false);
            connection.setUseCaches(false);
            connection.setConnectTimeout(5000);
            connection.setReadTimeout(5000);
            connection.setRequestProperty("Accept", "*/*");
            connection.setRequestProperty("Accept-Charset", "UTF-8");
            connection.setRequestProperty("Accept-Encoding", "identity");
            connection.setRequestProperty("Connection", "close");
            connection.setRequestProperty("User-Agent", "Mozilla/5.0 (compatible; MSIE 6.0; WIndows NT 5.1; SV1)");
            int code = connection.getResponseCode();
            if (code != 200) throw new IOException("HTTP " + code + " on " + resource.path + "; stopped without login or retry");
            try (InputStream input = connection.getInputStream(); ByteArrayOutputStream output = new ByteArrayOutputStream()) {
                byte[] buffer = new byte[8192];
                int count;
                while ((count = input.read(buffer)) != -1) {
                    if (Thread.currentThread().isInterrupted()) throw new IOException("Cancelled");
                    if (output.size() + count > MAX_BYTES) throw new IOException("Response exceeds 4 MiB");
                    output.write(buffer, 0, count);
                }
                return output.toByteArray();
            }
        } finally { connection.disconnect(); }
    }
}
