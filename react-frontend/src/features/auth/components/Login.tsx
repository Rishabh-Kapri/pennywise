import { ThemeToggle } from '@/components/theme/ThemeToggle';
import { useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import { loginWithGoogle, loginAsDemo, selectAuthLoading, selectAuthError, selectIsAuthenticated } from '@/features/auth/store/authSlice';
import { LoadingState, errorText, toast } from '@/utils';
import { config } from '@/config/env';
import { Check } from '@phosphor-icons/react';
import styles from './Login.module.css';
import { useGoogleLogin } from '@react-oauth/google';

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

function GoogleSignInButton() {
  const dispatch = useAppDispatch();
  const onGoogleLogin = useGoogleLogin({
    flow: 'auth-code', // the library adds `openid profile email`; Gmail is granted in Settings → Email sync
    // a failed sign-in shows under the buttons, from the auth slice
    onSuccess: (codeResponse) => void dispatch(loginWithGoogle(codeResponse.code)),
    onError: () => toast.error('Google sign-in failed'),
  });

  return <button className={styles.googleButton} onClick={() => onGoogleLogin()}>
    <LogoIcon />Sign In with Google
  </button>;
}

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

  const onDemoLogin = () => void dispatch(loginAsDemo());

  const isLoading = loading === LoadingState.PENDING;

  return (
    <div className={styles.container}>
      <img className={styles.backgroundImage} src="/images/budget-desk-envelopes.webp" alt="" />
      <div className={styles.toolbar}>
        <Link to="/">← Pennywise home</Link>
        <ThemeToggle />
      </div>
      <div className={styles.card}>
        {/* Logo */}
        <div className={styles.logoContainer}>
          <img className={styles.logoMark} src="/logo-mark.svg" alt="" />
          <span className={styles.logoText}>Pennywise</span>
        </div>

        {/* Welcome text */}
        <div className={styles.welcome}>
          <h1 className={styles.welcomeTitle}>
            Welcome <span className={styles.welcomeAccent}>back</span>
          </h1>
          <p className={styles.welcomeSubtitle}>Sign in to place, track, and trust every rupee.</p>
        </div>

        {/* Loading or Sign-in button */}
        {isLoading ? (
          <div className={styles.loading}>
            <div className={styles.spinner} />
            <span>Signing you in...</span>
          </div>
        ) : (
          <div className={styles.googleButtonContainer}>
            {!config.googleClientId && !config.demoMode && <div className={styles.error}>Google Login Not Enabled</div>}
            {config.googleClientId && (
              <GoogleSignInButton />
            )}
            {config.demoMode && (
              <button className={styles.demoButton} onClick={onDemoLogin}>
                Try Demo
              </button>
            )}
          </div>
        )}

        {/* Error */}
        {error && <div className={styles.error}>⚠️ {errorText(error)}</div>}

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
