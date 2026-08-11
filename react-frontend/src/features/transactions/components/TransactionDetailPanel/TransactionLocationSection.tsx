import { useMemo, useState } from 'react';
import { CrosshairIcon, MagnifyingGlassIcon, MapPinIcon, TrashIcon } from '@phosphor-icons/react';
import { MapContainer, Marker, TileLayer, useMapEvents } from 'react-leaflet';
import L from 'leaflet';
import 'leaflet/dist/leaflet.css';
import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png';
import markerIcon from 'leaflet/dist/images/marker-icon.png';
import markerShadow from 'leaflet/dist/images/marker-shadow.png';
import { useAppDispatch } from '@/app/hooks';
import { apiClient } from '@/utils/api';
import { updateTransactionLocation } from '../../store/transactionSlice';
import { LocationSource, type Transaction } from '../../types/transaction.types';
import styles from './TransactionDetailPanel.module.css';

// Vite bundling breaks leaflet's default icon path resolution; the legacy
// _getIconUrl ignores merged options, so it has to go first
delete (L.Icon.Default.prototype as unknown as Record<string, unknown>)._getIconUrl;
L.Icon.Default.mergeOptions({
  iconRetinaUrl: markerIcon2x,
  iconUrl: markerIcon,
  shadowUrl: markerShadow,
});

// fallback map center when picking a pin with no location yet (India)
const DEFAULT_CENTER: [number, number] = [20.5937, 78.9629];

type PlaceResult = { name: string; lat: number; lng: number };

// Overridable so a blocked or poor basemap is an env change rather than a
// release. OSM's volunteer tile servers require a Referer and are explicitly
// not intended for application traffic; MapTiler and Mapbox free tiers take a
// key in the URL and are far more dependable.
const TILE_URL =
  import.meta.env.VITE_MAP_TILE_URL || 'https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
const TILE_ATTRIBUTION =
  import.meta.env.VITE_MAP_TILE_ATTRIBUTION ||
  '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors';

function PinPicker({ onPick }: { onPick: (lat: number, lng: number) => void }) {
  useMapEvents({
    click(event) {
      onPick(event.latlng.lat, event.latlng.lng);
    },
  });
  return null;
}

export function TransactionLocationSection({ txn }: { txn: Transaction }) {
  const dispatch = useAppDispatch();
  const [isPicking, setIsPicking] = useState(false);
  const [draftPin, setDraftPin] = useState<[number, number] | null>(null);
  const [isLocating, setIsLocating] = useState(false);
  const [localError, setLocalError] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<PlaceResult[]>([]);
  const [isSearching, setIsSearching] = useState(false);

  const hasLocation = txn.locationLat != null && txn.locationLng != null;
  const position = useMemo<[number, number] | null>(
    () => (hasLocation ? [txn.locationLat as number, txn.locationLng as number] : null),
    [hasLocation, txn.locationLat, txn.locationLng],
  );

  if (!txn.id) return null;

  // `name` is passed through when it came from a search hit, so the server does
  // not reverse-geocode a coordinate we were already given a name for.
  const saveLocation = (lat: number, lng: number, name?: string) => {
    setLocalError(null);
    dispatch(
      updateTransactionLocation({
        id: txn.id!,
        location: { lat, lng, name, source: LocationSource.MANUAL },
      }),
    );
    setIsPicking(false);
    setDraftPin(null);
    setResults([]);
    setQuery('');
  };

  const searchPlaces = async (event: React.FormEvent) => {
    event.preventDefault();
    const q = query.trim();
    if (!q) return;
    setIsSearching(true);
    setLocalError(null);
    try {
      const found = await apiClient.get<PlaceResult[]>(`geocode/search?q=${encodeURIComponent(q)}`);
      setResults(Array.isArray(found) ? found : []);
      if (!found?.length) setLocalError('No places found');
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : 'Search failed');
      setResults([]);
    } finally {
      setIsSearching(false);
    }
  };

  const removeLocation = () => {
    setLocalError(null);
    dispatch(updateTransactionLocation({ id: txn.id!, location: { lat: null, lng: null } }));
    setIsPicking(false);
    setDraftPin(null);
  };

  const useMyLocation = () => {
    if (!navigator.geolocation) {
      setLocalError('Geolocation is not supported by this browser');
      return;
    }
    setIsLocating(true);
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        setIsLocating(false);
        saveLocation(pos.coords.latitude, pos.coords.longitude);
      },
      (err) => {
        setIsLocating(false);
        setLocalError(err.message || 'Could not read your location');
      },
      { enableHighAccuracy: true, timeout: 10_000 },
    );
  };

  const showMap = hasLocation || isPicking;
  const mapCenter = draftPin ?? position ?? DEFAULT_CENTER;
  const markerPosition = draftPin ?? position;

  return (
    <section className={styles.locationSection}>
      <span className={styles.metaLabel}>
        <MapPinIcon size={18} />
        <span>Location</span>
        {txn.locationSource === LocationSource.AUTO && (
          <span className={styles.autoBadge} title="Attached automatically from your phone's location">
            auto
          </span>
        )}
      </span>

      {hasLocation && !isPicking && (
        <div className={styles.locationNameRow}>
          <span className={styles.locationName}>
            {txn.locationName || `${txn.locationLat?.toFixed(5)}, ${txn.locationLng?.toFixed(5)}`}
          </span>
        </div>
      )}

      {isPicking && (
        <div className={styles.locationSearch}>
          <form onSubmit={searchPlaces} className={styles.locationSearchForm}>
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search a place"
              className={styles.locationSearchInput}
            />
            <button type="submit" className={styles.locationBtnSecondary} disabled={isSearching}>
              <MagnifyingGlassIcon size={14} />
              {isSearching ? 'Searching…' : 'Search'}
            </button>
          </form>
          {results.length > 0 && (
            <ul className={styles.locationResults}>
              {results.map((place) => (
                <li key={`${place.lat},${place.lng}`}>
                  <button
                    type="button"
                    className={styles.locationResult}
                    onClick={() => saveLocation(place.lat, place.lng, place.name)}>
                    <MapPinIcon size={13} />
                    <span>{place.name}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {showMap && (
        <div className={styles.locationMapWrap}>
          <MapContainer
            key={`${mapCenter[0]},${mapCenter[1]},${isPicking}`}
            center={mapCenter}
            zoom={hasLocation || draftPin ? 16 : 5}
            className={styles.locationMap}
            scrollWheelZoom={false}>
            <TileLayer attribution={TILE_ATTRIBUTION} url={TILE_URL} />
            {markerPosition && <Marker position={markerPosition} />}
            {isPicking && <PinPicker onPick={(lat, lng) => setDraftPin([lat, lng])} />}
          </MapContainer>
          {isPicking && <span className={styles.locationHint}>Click the map to place the pin</span>}
        </div>
      )}

      <div className={styles.locationActions}>
        {isPicking ? (
          <>
            <button
              type="button"
              className={styles.locationBtn}
              disabled={!draftPin}
              onClick={() => draftPin && saveLocation(draftPin[0], draftPin[1])}>
              Save pin
            </button>
            <button
              type="button"
              className={styles.locationBtnSecondary}
              onClick={() => {
                setIsPicking(false);
                setDraftPin(null);
              }}>
              Cancel
            </button>
          </>
        ) : (
          <>
            <button type="button" className={styles.locationBtn} onClick={() => setIsPicking(true)}>
              <MapPinIcon size={14} />
              {hasLocation ? 'Move pin' : 'Pick on map'}
            </button>
            <button
              type="button"
              className={styles.locationBtnSecondary}
              onClick={useMyLocation}
              disabled={isLocating}>
              <CrosshairIcon size={14} />
              {isLocating ? 'Locating…' : 'Use my location'}
            </button>
            {hasLocation && (
              <button type="button" className={styles.locationBtnDanger} onClick={removeLocation}>
                <TrashIcon size={14} />
                Remove
              </button>
            )}
          </>
        )}
      </div>

      {localError && <span className={styles.errorText}>{localError}</span>}
    </section>
  );
}
