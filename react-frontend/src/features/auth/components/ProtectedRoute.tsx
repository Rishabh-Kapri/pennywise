import { Navigate, useLocation } from 'react-router-dom';
import { useAppSelector } from '@/app/hooks';
import { selectAuthLoading, selectIsAuthenticated } from '@/features/auth/store/authSlice';
import { LoadingState } from '@/utils';
import { AppLoader } from '@/components/common/AppLoader/AppLoader';

interface ProtectedRouteProps {
  children: React.ReactNode;
}

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const isAuthenticated = useAppSelector(selectIsAuthenticated);
  const loading = useAppSelector(selectAuthLoading);
  const location = useLocation();

  // Show loading state while checking auth
  if (loading === LoadingState.PENDING) {
    return <AppLoader fullScreen label="Signing you in" />;
  }

  // Redirect to login if not authenticated
  if (!isAuthenticated) {
    // Save the attempted location for redirect after login
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
}

export default ProtectedRoute;
