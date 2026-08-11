import { useCallback, useMemo, useRef, useState } from 'react';
import { ActivityIndicator, Modal, Pressable, StyleSheet, TextInput, View } from 'react-native';
import { WebView, type WebViewMessageEvent } from 'react-native-webview';
import { MapPin, Search, X } from 'lucide-react-native';
import { AppText } from '../../../components/AppText';
import { Button } from '../../../components/Button';
import { apiClient } from '../../../utils/api';
import { colors, radii, spacing } from '../../../theme';

type Coords = { lat: number; lng: number };

type PlaceResult = {
  name: string;
  lat: number;
  lng: number;
};

/** Centre of India, used when there is nothing to centre on yet. */
const DEFAULT_CENTER: Coords = { lat: 20.5937, lng: 78.9629 };

/**
 * Leaflet in a WebView rather than a native map module.
 *
 * Keeps the app free of a Google Maps API key and matches the web app's tiles
 * and interaction exactly. Dark CARTO basemap so the map sits inside the app's
 * palette instead of glowing white in the middle of it.
 */
function buildHtml(center: Coords, hasPin: boolean): string {
  return `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no" />
  <link rel="stylesheet" href="https://unpkg.com/leaflet@1.9.4/dist/leaflet.css" />
  <script src="https://unpkg.com/leaflet@1.9.4/dist/leaflet.js"></script>
  <style>
    html, body, #map { height: 100%; margin: 0; background: ${colors.background}; }
    .leaflet-control-attribution { font-size: 9px; background: rgba(0,0,0,0.5); color: #9C9CA6; }
    .leaflet-control-attribution a { color: ${colors.primary}; }
  </style>
</head>
<body>
  <div id="map"></div>
  <script>
    var map = L.map('map', { zoomControl: true, attributionControl: true })
      .setView([${center.lat}, ${center.lng}], ${hasPin ? 16 : 5});

    L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
      maxZoom: 19,
      attribution: '&copy; OpenStreetMap &copy; CARTO'
    }).addTo(map);

    var marker = ${hasPin} ? L.marker([${center.lat}, ${center.lng}]).addTo(map) : null;

    function post(lat, lng) {
      window.ReactNativeWebView.postMessage(JSON.stringify({ type: 'pick', lat: lat, lng: lng }));
    }

    function place(lat, lng) {
      if (marker) { marker.setLatLng([lat, lng]); }
      else { marker = L.marker([lat, lng]).addTo(map); }
      post(lat, lng);
    }

    map.on('click', function (e) { place(e.latlng.lat, e.latlng.lng); });

    // Called from React Native when a search result is chosen.
    window.gotoPlace = function (lat, lng) {
      map.setView([lat, lng], 16);
      place(lat, lng);
    };
  </script>
</body>
</html>`;
}

export function LocationPickerModal({
  visible,
  initial,
  onClose,
  onPick
}: {
  visible: boolean;
  initial: Coords | null;
  onClose: () => void;
  onPick: (coords: Coords, name: string | null) => void;
}) {
  const webRef = useRef<WebView>(null);
  const [pin, setPin] = useState<Coords | null>(initial);
  // Set when a pin came from a search hit, so the place name is kept rather
  // than making the server reverse-geocode a coordinate it already had a name for.
  const [pinName, setPinName] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<PlaceResult[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Built once per open: re-rendering the HTML would reset the map view and
  // throw away the user's panning.
  const html = useMemo(
    () => buildHtml(initial ?? DEFAULT_CENTER, initial != null),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [visible]
  );

  const onMessage = useCallback((event: WebViewMessageEvent) => {
    try {
      const data = JSON.parse(event.nativeEvent.data) as { type: string; lat: number; lng: number };
      if (data.type !== 'pick') return;
      setPin({ lat: data.lat, lng: data.lng });
      // A tap on the map is a new place, so any name from a previous search
      // no longer describes it.
      setPinName(null);
    } catch {
      // Ignore anything that isn't our message shape.
    }
  }, []);

  const search = async () => {
    const q = query.trim();
    if (!q) return;
    setIsSearching(true);
    setError(null);
    try {
      const found = await apiClient.get<PlaceResult[]>(`geocode/search?q=${encodeURIComponent(q)}`);
      setResults(Array.isArray(found) ? found : []);
      if (!found?.length) setError('No places found');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed');
      setResults([]);
    } finally {
      setIsSearching(false);
    }
  };

  const choose = (place: PlaceResult) => {
    setResults([]);
    setQuery(place.name);
    setPin({ lat: place.lat, lng: place.lng });
    setPinName(place.name);
    webRef.current?.injectJavaScript(`window.gotoPlace(${place.lat}, ${place.lng}); true;`);
  };

  return (
    <Modal visible={visible} animationType="slide" onRequestClose={onClose} statusBarTranslucent>
      <View style={styles.screen}>
        <View style={styles.header}>
          <AppText variant="heading">Pick a location</AppText>
          <Pressable onPress={onClose} hitSlop={10} style={styles.close}>
            <X size={20} color={colors.muted} />
          </Pressable>
        </View>

        <View style={styles.searchRow}>
          <Search size={16} color={colors.faint} />
          <TextInput
            value={query}
            onChangeText={setQuery}
            onSubmitEditing={() => void search()}
            returnKeyType="search"
            placeholder="Search a place"
            placeholderTextColor={colors.faint}
            style={styles.searchInput}
          />
          {isSearching ? <ActivityIndicator size="small" color={colors.primary} /> : null}
        </View>

        {error ? (
          <AppText variant="caption" tone="danger" style={styles.error}>
            {error}
          </AppText>
        ) : null}

        {results.length > 0 ? (
          <View style={styles.results}>
            {results.map((place) => (
              <Pressable
                key={`${place.lat},${place.lng}`}
                onPress={() => choose(place)}
                android_ripple={{ color: colors.surfaceTertiary }}
                style={styles.resultRow}
              >
                <MapPin size={14} color={colors.primary} />
                <AppText variant="caption" numberOfLines={2} style={styles.resultText}>
                  {place.name}
                </AppText>
              </Pressable>
            ))}
          </View>
        ) : null}

        <View style={styles.mapWrap}>
          <WebView
            ref={webRef}
            source={{ html }}
            originWhitelist={['*']}
            onMessage={onMessage}
            style={styles.map}
            // The map is the only content; nothing to scroll around it.
            scrollEnabled={false}
          />
        </View>

        <View style={styles.footer}>
          <AppText variant="caption" muted numberOfLines={1} style={styles.coords}>
            {pin ? `${pin.lat.toFixed(5)}, ${pin.lng.toFixed(5)}` : 'Tap the map to drop a pin'}
          </AppText>
          <Button
            size="sm"
            disabled={!pin}
            onPress={() => {
              if (pin) onPick(pin, pinName);
            }}
          >
            Save pin
          </Button>
        </View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: colors.background, paddingTop: spacing.xl },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: spacing.xl,
    paddingBottom: spacing.md
  },
  close: { padding: spacing.xs },
  searchRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    marginHorizontal: spacing.xl,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
    borderRadius: radii.full,
    backgroundColor: colors.surfaceStrong
  },
  searchInput: { flex: 1, color: colors.text, paddingVertical: 0 },
  error: { marginHorizontal: spacing.xl, marginTop: spacing.xs },
  results: {
    marginHorizontal: spacing.xl,
    marginTop: spacing.sm,
    borderRadius: radii.lg,
    backgroundColor: colors.surface,
    overflow: 'hidden'
  },
  resultRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm
  },
  resultText: { flex: 1 },
  mapWrap: {
    flex: 1,
    margin: spacing.xl,
    borderRadius: radii.lg,
    overflow: 'hidden',
    backgroundColor: colors.surface
  },
  map: { flex: 1, backgroundColor: colors.background },
  footer: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: spacing.md,
    paddingHorizontal: spacing.xl,
    paddingBottom: spacing.xl
  },
  coords: { flex: 1 }
});
