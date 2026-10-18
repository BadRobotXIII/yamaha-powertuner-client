package local.powertuner.readonly;

import android.app.Activity;
import android.os.Bundle;
import android.net.ConnectivityManager;
import android.net.Network;
import android.net.NetworkCapabilities;
import android.net.LinkProperties;
import android.content.ClipboardManager;
import android.content.ClipData;
import android.content.Intent;
import android.graphics.Typeface;
import android.widget.*;
import java.net.HttpURLConnection;
import java.util.ArrayList;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.nio.charset.StandardCharsets;
import java.io.OutputStream;
import org.json.JSONObject;

public final class MainActivity extends Activity {
    private final ExecutorService worker = Executors.newSingleThreadExecutor();
    private final ArrayList<Network> networks = new ArrayList<>();
    private Spinner wifi, endpoint, format, response;
    private EditText address;
    private CheckBox eps;
    private TextView status, output;
    private Button read;
    private volatile boolean destroyed;
    private String result = "";
    private final ArrayList<byte[]> bodies = new ArrayList<>();
    private byte[] pendingSave;

    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        LinearLayout layout = new LinearLayout(this);
        layout.setOrientation(LinearLayout.VERTICAL);
        int pad = (int) (16 * getResources().getDisplayMetrics().density);
        layout.setPadding(pad, pad, pad, pad);
        layout.setOnApplyWindowInsetsListener((view, insets) -> {
            view.setPadding(pad + insets.getSystemWindowInsetLeft(), pad + insets.getSystemWindowInsetTop(),
                pad + insets.getSystemWindowInsetRight(), pad + insets.getSystemWindowInsetBottom());
            return insets;
        });
        TextView title = new TextView(this); title.setText("CCU Read Only"); title.setTextSize(24); layout.addView(title);
        TextView help = new TextView(this); help.setText("Join the CCU Wi-Fi in Android Settings, then refresh and select it below. Each tap reads one snapshot. Map values stay raw."); layout.addView(help);
        address = new EditText(this); address.setSingleLine(); address.setInputType(17); address.setText("http://192.168.0.1:8080"); layout.addView(address);
        wifi = new Spinner(this); layout.addView(wifi);
        Button refresh = new Button(this); refresh.setText("Refresh Wi-Fi networks"); refresh.setOnClickListener(v -> refreshNetworks()); layout.addView(refresh);
        endpoint = new Spinner(this);
        endpoint.setAdapter(new ArrayAdapter<>(this, android.R.layout.simple_spinner_dropdown_item,
            new String[]{"Identity", "Maps", "Telemetry / diagnostics", "ECU model", "Lock status", "Laps", "Activity log 0", "Activity log 1", "Activity log 2", "Activity log 3", "EPS settings"}));
        layout.addView(endpoint);
        eps = new CheckBox(this); eps.setText("Include EPS with maps (supported vehicles only)"); layout.addView(eps);
        format = new Spinner(this);
        format.setAdapter(new ArrayAdapter<>(this, android.R.layout.simple_spinner_dropdown_item,
            new String[]{"JSON / UTF-8 text", "Binary (bits)", "Hex", "JSON / Binary / Hex"}));
        layout.addView(format);
        response = new Spinner(this); layout.addView(response);
        android.widget.AdapterView.OnItemSelectedListener displayChange = new android.widget.AdapterView.OnItemSelectedListener() {
            public void onItemSelected(android.widget.AdapterView<?> parent, android.view.View view, int position, long id) { renderResponse(); }
            public void onNothingSelected(android.widget.AdapterView<?> parent) { }
        };
        format.setOnItemSelectedListener(displayChange); response.setOnItemSelectedListener(displayChange);
        read = new Button(this); read.setText("Read snapshot"); read.setOnClickListener(v -> readSnapshot()); layout.addView(read);
        Button copy = new Button(this); copy.setText("Copy result"); copy.setOnClickListener(v -> {
            ((ClipboardManager) getSystemService(CLIPBOARD_SERVICE)).setPrimaryClip(ClipData.newPlainText("CCU result", result));
        }); layout.addView(copy);
        Button save = new Button(this); save.setText("Save original bytes (.bin)"); save.setOnClickListener(v -> saveResponse()); layout.addView(save);
        status = new TextView(this); layout.addView(status);
        output = new TextView(this); output.setTypeface(Typeface.MONOSPACE); output.setTextSize(12); output.setTextIsSelectable(true);
        output.setMinHeight(pad * 12); layout.addView(output);
        ScrollView scroll = new ScrollView(this); scroll.setFillViewport(true); scroll.addView(layout);
        setContentView(scroll);
        refreshNetworks(); // Enumerates Android networks only; sends no HTTP requests.
    }

    private void refreshNetworks() {
        networks.clear();
        ArrayList<String> names = new ArrayList<>();
        ConnectivityManager manager = (ConnectivityManager) getSystemService(CONNECTIVITY_SERVICE);
        for (Network network : manager.getAllNetworks()) {
            NetworkCapabilities caps = manager.getNetworkCapabilities(network);
            if (caps != null && caps.hasTransport(NetworkCapabilities.TRANSPORT_WIFI)) {
                networks.add(network);
                LinkProperties links = manager.getLinkProperties(network);
                names.add("Wi-Fi " + network + " " + (links == null ? "" : links.getLinkAddresses()));
            }
        }
        if (names.isEmpty()) names.add("No connected Wi-Fi; join in Android Settings");
        wifi.setAdapter(new ArrayAdapter<>(this, android.R.layout.simple_spinner_dropdown_item, names));
        status.setText("Idle. No requests sent. GET only; no login or uploads.");
    }

    private void readSnapshot() {
        int selected = wifi.getSelectedItemPosition();
        if (selected < 0 || selected >= networks.size()) { status.setText("Connect to Wi-Fi first, then refresh."); return; }
        final Network network = networks.get(selected);
        final String host = address.getText().toString().trim();
        final int command = endpoint.getSelectedItemPosition();
        final boolean includeEPS = eps.isChecked();
        read.setEnabled(false); status.setText("Reading through Wi-Fi " + network + "...");
        worker.execute(() -> {
            String message;
            ArrayList<byte[]> captured = new ArrayList<>();
            ArrayList<String> labels = new ArrayList<>();
            try {
                ReadOnlyClient client = new ReadOnlyClient(host, url -> (HttpURLConnection) network.openConnection(url, java.net.Proxy.NO_PROXY));
                if (command == 1) {
                    byte[] identityBytes = client.readBytes(ReadOnlyClient.Resource.INFO);
                    captured.add(identityBytes); labels.add("/info/");
                    String identity = new String(identityBytes, StandardCharsets.UTF_8);
                    if (new JSONObject(identity).optString("SerialNumber", "").isEmpty()) throw new Exception("No SerialNumber in identity; map read stopped");
                    captured.add(client.readBytes(ReadOnlyClient.Resource.MAPS)); labels.add("/ecu/settings/");
                    if (includeEPS) {
                        captured.add(client.readBytes(ReadOnlyClient.Resource.EPS)); labels.add("/eps/settings/");
                    }
                } else {
                    ReadOnlyClient.Resource[] resources = {ReadOnlyClient.Resource.INFO, ReadOnlyClient.Resource.MAPS,
                        ReadOnlyClient.Resource.MONITOR, ReadOnlyClient.Resource.MODEL, ReadOnlyClient.Resource.LOCK,
                        ReadOnlyClient.Resource.LAPS, ReadOnlyClient.Resource.LOG0, ReadOnlyClient.Resource.LOG1,
                        ReadOnlyClient.Resource.LOG2, ReadOnlyClient.Resource.LOG3, ReadOnlyClient.Resource.EPS};
                    captured.add(client.readBytes(resources[command])); labels.add(resources[command].name());
                }
                message = "Read complete. Select response and display format; no conversion of ECU values.";
            } catch (Exception e) { message = "Read stopped: " + e.getMessage() + ". Any earlier responses are retained."; }
            final String finalMessage = message;
            runOnUiThread(() -> { if (!destroyed) {
                bodies.clear(); bodies.addAll(captured);
                response.setAdapter(new ArrayAdapter<>(this, android.R.layout.simple_spinner_dropdown_item, labels));
                response.setSelection(bodies.size() > 1 ? 1 : 0);
                renderResponse(); status.setText(finalMessage); read.setEnabled(true);
            } });
        });
    }
    private void renderResponse() {
        if (output == null) return;
        int index = response.getSelectedItemPosition();
        if (index < 0 || index >= bodies.size()) { result = ""; output.setText(result); return; }
        byte[] body = bodies.get(index);
        result = body.length + " response-body bytes\n\n" + ResponseFormatter.format(body,
            ResponseFormatter.Mode.values()[Math.max(0, format.getSelectedItemPosition())]);
        output.setText(result);
    }
    private void saveResponse() {
        int index = response.getSelectedItemPosition();
        if (index < 0 || index >= bodies.size()) { status.setText("Read a response first."); return; }
        pendingSave = bodies.get(index).clone();
        Intent intent = new Intent(Intent.ACTION_CREATE_DOCUMENT);
        intent.addCategory(Intent.CATEGORY_OPENABLE); intent.setType("application/octet-stream");
        intent.putExtra(Intent.EXTRA_TITLE, "ccu-response.bin"); startActivityForResult(intent, 1);
    }
    @Override protected void onActivityResult(int request, int code, Intent data) {
        super.onActivityResult(request, code, data);
        if (request != 1) return;
        byte[] saved = pendingSave; pendingSave = null;
        if (code != RESULT_OK || data == null || data.getData() == null || saved == null) return;
        final android.net.Uri target = data.getData();
        worker.execute(() -> {
            String message;
            try (OutputStream stream = getContentResolver().openOutputStream(target, "wt")) {
                if (stream == null) throw new Exception("Cannot open destination");
                stream.write(saved); message = "Saved " + saved.length + " original response-body bytes.";
            } catch (Exception e) { message = "Save failed: " + e.getMessage(); }
            final String finalMessage = message;
            runOnUiThread(() -> { if (!destroyed) status.setText(finalMessage); });
        });
    }
    @Override public void onDestroy() { destroyed = true; worker.shutdownNow(); super.onDestroy(); }
}
