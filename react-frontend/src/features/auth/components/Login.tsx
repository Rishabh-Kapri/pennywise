import { useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import {
  loginWithGoogle,
  selectAuthLoading,
  selectAuthError,
  selectIsAuthenticated,
  clearError,
} from '../store';
import { LoadingState, toast } from '@/utils';
import { Check } from 'lucide-react';
import styles from './Login.module.css';

// Google OAuth Client ID
const GOOGLE_CLIENT_ID = import.meta.env.VITE_GOOGLE_CLIENT_ID || '';

// Google Identity Services types
declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (config: {
            client_id: string;
            callback: (response: { credential: string }) => void;
            auto_select?: boolean;
          }) => void;
          renderButton: (
            element: HTMLElement,
            options: {
              theme?: 'outline' | 'filled_blue' | 'filled_black';
              size?: 'large' | 'medium' | 'small';
              text?: 'signin_with' | 'signup_with' | 'continue_with' | 'signin';
              width?: number;
              logo_alignment?: 'left' | 'center';
            }
          ) => void;
          prompt: () => void;
        };
      };
    };
  }
}

// Logo SVG component matching the design
const LogoIcon = () => (
  <svg
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    className={styles.logoIcon}
  >
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
    if (isAuthenticated) {
      navigate('/', { replace: true });
    }
  }, [isAuthenticated, navigate]);

  const handleGoogleCallback = useCallback(
    (response: { credential: string }) => {
      dispatch(loginWithGoogle(response.credential))
        .unwrap()
        .catch((err: unknown) => {
          const message = err instanceof Error ? err.message : typeof err === 'string' ? err : 'Login failed';
          toast.error(message);
        });
    },
    [dispatch]
  );

  useEffect(() => {
    dispatch(clearError());

    const script = document.createElement('script');
    script.src = 'https://accounts.google.com/gsi/client';
    script.async = true;
    script.defer = true;
    script.onload = () => {
      if (window.google && GOOGLE_CLIENT_ID) {
        window.google.accounts.id.initialize({
          client_id: GOOGLE_CLIENT_ID,
          callback: handleGoogleCallback,
        });

        const buttonContainer = document.getElementById('google-signin-button');
        if (buttonContainer) {
          window.google.accounts.id.renderButton(buttonContainer, {
            theme: 'filled_black',
            size: 'large',
            text: 'continue_with',
            width: 320,
          });
        }
      }
    };
    document.body.appendChild(script);

    return () => {
      const existingScript = document.querySelector(
        'script[src="https://accounts.google.com/gsi/client"]'
      );
      if (existingScript) {
        existingScript.remove();
      }
    };
  }, [dispatch, handleGoogleCallback]);

  const isLoading = loading === LoadingState.PENDING;

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        {/* Logo */}
        <div className={styles.logoContainer}>
          <LogoIcon />
          <span className={styles.logoText}>Pennywise</span>
        </div>

        {/* Welcome text */}
        <div className={styles.welcome}>
          <h1 className={styles.welcomeTitle}>Welcome back</h1>
          <p className={styles.welcomeSubtitle}>Sign in to manage your budget</p>
        </div>

        {/* Loading or Sign-in button */}
        {isLoading ? (
          <div className={styles.loading}>
            <div className={styles.spinner} />
            <span>Signing you in...</span>
          </div>
        ) : (
          <div className={styles.googleButtonContainer}>
            <div id="google-signin-button" />
            
            {!GOOGLE_CLIENT_ID && (
              <div className={styles.error}>
                ⚠️ Set VITE_GOOGLE_CLIENT_ID in environment
              </div>
            )}
          </div>
        )}

        {/* Error */}
        {error && (
          <div className={styles.error}>⚠️ {error}</div>
        )}

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
          By continuing, you agree to our{' '}
          <a href="#">Terms of Service</a> and <a href="#">Privacy Policy</a>
        </div>
      </div>
    </div>
  );
}

export default Login;
