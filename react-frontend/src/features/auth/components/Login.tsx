import { useEffect, useCallback } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import {
  loginWithGoogle,
  selectAuthLoading,
  selectAuthError,
  selectIsAuthenticated,
} from '../store';
import { LoadingState, toast } from '@/utils';
import { Check } from 'lucide-react';
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

export function Login() {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const loading = useAppSelector(selectAuthLoading);
  const error = useAppSelector(selectAuthError);
  const isAuthenticated = useAppSelector(selectIsAuthenticated);

  useEffect(() => {
    console.log(JSON.stringify(import.meta.env.VITE_GOOGLE_CLIENT_ID));
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
      console.log('inside onGoogleLogin', codeResponse);
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
      <div className={styles.orbOne} aria-hidden="true" />
      <div className={styles.orbTwo} aria-hidden="true" />
      <div className={styles.card}>
        {/* Logo */}
        <div className={styles.logoContainer}>
          <LogoIcon />
          <span className={styles.logoText}>Pennywise</span>
        </div>

        {/* Welcome text */}
        <div className={styles.welcome}>
          <h1 className={styles.welcomeTitle}>Welcome back</h1>
          <p className={styles.welcomeSubtitle}>
            Sign in to manage your budget
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
      </div>
    </div>
  );
}

export default Login;
