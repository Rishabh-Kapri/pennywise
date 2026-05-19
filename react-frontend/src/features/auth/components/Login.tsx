import { useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import {
  loginWithGoogle,
  selectAuthLoading,
  selectAuthError,
  selectIsAuthenticated,
} from '../store';
import { LoadingState, toast } from '@/utils';
import { Check } from '@phosphor-icons/react';
import styles from './Login.module.css';
import { useGoogleLogin } from '@react-oauth/google';

// Google OAuth Client ID
const GOOGLE_CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID || '';

// Google Identity Services types

// Logo SVG component matching the design
const LogoIcon = () => (
  <svg
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={styles.logoIcon}>
    <path d="M12 2L2 7l10 5 10-5-10-5z" />
    <path d="M2 17l10 5 10-5" />
    <path d="M2 12l10 5 10-5" />
  </svg>
);

/* ── Floating vector glyph SVGs ── */

const GlyphRing = ({ size = 36 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 36 36" fill="none" xmlns="http://www.w3.org/2000/svg">
    <circle cx="18" cy="18" r="15" stroke="currentColor" strokeWidth="1.5" />
    <circle cx="18" cy="18" r="7" stroke="currentColor" strokeWidth="1" opacity="0.4" />
  </svg>
);

const GlyphCross = ({ size = 22 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 22 22" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M11 3v16M3 11h16" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
  </svg>
);

const GlyphDiamond = ({ size = 28 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 28 28" fill="none" xmlns="http://www.w3.org/2000/svg">
    <rect x="14" y="2" width="16" height="16" rx="2.5" transform="rotate(45 14 2)" stroke="currentColor" strokeWidth="1.5" />
  </svg>
);

const GlyphHex = ({ size = 30 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 30 30" fill="none" xmlns="http://www.w3.org/2000/svg">
    <path d="M15 3L26 9.5v11L15 27 4 20.5v-11L15 3Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
  </svg>
);

export function Login() {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const loading = useAppSelector(selectAuthLoading);
  const error = useAppSelector(selectAuthError);
  const isAuthenticated = useAppSelector(selectIsAuthenticated);

  useEffect(() => {
    if (isAuthenticated) {
      navigate('/dashboard', { replace: true });
    }
  }, [isAuthenticated, navigate]);

  // const handleGoogleCallback = useCallback(
  //   (response: { credential: string }) => {
  //     dispatch(loginWithGoogle(response.credential))
  //       .unwrap()
  //       .catch((err: unknown) => {
  //         const message =
  //           err instanceof Error
  //             ? err.message
  //             : typeof err === 'string'
  //               ? err
  //               : 'Login failed';
  //         toast.error(message);
  //       });
  //   },
  //   [dispatch],
  // );

  const onGoogleLogin = useGoogleLogin({
    flow: 'auth-code',
    scope: 'https://mail.google.com/',
    onSuccess: async (codeResponse) => {
      dispatch(loginWithGoogle(codeResponse.code))
        .unwrap()
        .catch((err: unknown) => {
          const message =
            err instanceof Error
              ? err.message
              : typeof err === 'string'
                ? err
                : 'Login failed';
          toast.error(message);
        });
    },
    onError: (error) => {
      console.log('inside onError', error);
    },
  });

const isLoading = loading === LoadingState.PENDING;

  return (
    <div className={styles.container}>
      {/* Dot-grid background */}
      <div className={styles.bgDots} aria-hidden="true" />

      {/* Floating vector glyphs */}
      <div className={`${styles.glyphFloat} ${styles.glyph1}`} aria-hidden="true">
        <GlyphRing size={40} />
      </div>
      <div className={`${styles.glyphFloat} ${styles.glyph2}`} aria-hidden="true">
        <GlyphCross size={22} />
      </div>
      <div className={`${styles.glyphFloat} ${styles.glyph3}`} aria-hidden="true">
        <GlyphDiamond size={30} />
      </div>
      <div className={`${styles.glyphFloat} ${styles.glyph4}`} aria-hidden="true">
        <GlyphHex size={34} />
      </div>

      <div className={styles.card}>
        {/* Logo */}
        <div className={styles.logoContainer}>
          <span className={styles.logoMark}>P</span>
          <span className={styles.logoText}>Pennywise</span>
        </div>

        {/* Welcome text */}
        <div className={styles.welcome}>
          <h1 className={styles.welcomeTitle}>Welcome <span className={styles.welcomeAccent}>back</span></h1>
          <p className={styles.welcomeSubtitle}>
            Sign in to place, track, and trust every rupee.
          </p>
        </div>

        {/* Loading or Sign-in button */}
        {isLoading ? (
          <div className={styles.loading}>
            <div className={styles.spinner} />
            <span>Signing you in...</span>
          </div>
        ) : (
          <div className={styles.googleButtonContainer}>
            {!GOOGLE_CLIENT_ID && (
              <div className={styles.error}>Google Login Not Enabled</div>
            )}
            {GOOGLE_CLIENT_ID && (
              <button className={styles.googleButton} onClick={onGoogleLogin}>
                <LogoIcon />
                Sign In with Google
              </button>
            )}
          </div>
        )}

        {/* Error */}
        {error && <div className={styles.error}>⚠️ {error}</div>}

        {/* Features */}
        <div className={styles.features}>
          <div className={styles.featuresTitle}>What you get</div>
          <div className={styles.featuresList}>
            <div className={styles.featureItem}>
              <Check size={18} className={styles.featureIcon} />
              <span>Zero-based budgeting</span>
            </div>
            <div className={styles.featureItem}>
              <Check size={18} className={styles.featureIcon} />
              <span>Automatic transaction categorization</span>
            </div>
            <div className={styles.featureItem}>
              <Check size={18} className={styles.featureIcon} />
              <span>Spending insights & reports</span>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className={styles.footer}>
          By continuing, you agree to our <Link to="/terms">Terms of Service</Link> and{' '}
          <Link to="/privacy">Privacy Policy</Link>
        </div>

        <Link to="/" className={styles.homeLink}>
          Back to homepage
        </Link>
      </div>
    </div>
  );
}

export default Login;
