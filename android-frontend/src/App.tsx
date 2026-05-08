import 'react-native-gesture-handler';
import { useEffect } from 'react';
import { StyleSheet, View } from 'react-native';
import { Provider } from 'react-redux';
import { DarkTheme, NavigationContainer } from '@react-navigation/native';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { SafeAreaProvider } from 'react-native-safe-area-context';
import { StatusBar } from 'expo-status-bar';
import { BarChart3, Landmark, LayoutDashboard, ReceiptText, Settings, Tags } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from './app/hooks';
import { store } from './app/store';
import { LoadingStateView } from './components/LoadingStateView';
import { colors } from './theme';
import { apiClient } from './utils/api';
import type { AppTabParamList, AuthStackParamList } from './navigation/types';
import { hydrateAuth } from './features/auth/store/authSlice';
import { LoginScreen } from './features/auth/screens/LoginScreen';
import { fetchAllBudgets } from './features/budget/store/budgetSlice';
import { BudgetOnboardingScreen } from './features/budget/screens/BudgetOnboardingScreen';
import { DashboardScreen } from './features/dashboard/screens/DashboardScreen';
import { BudgetScreen } from './features/budget/screens/BudgetScreen';
import { TransactionsScreen } from './features/transactions/screens/TransactionsScreen';
import { PayeesScreen } from './features/payees/screens/PayeesScreen';
import { LoansScreen } from './features/loans/screens/LoansScreen';
import { SettingsScreen } from './features/settings/screens/SettingsScreen';
import { WebSocketProvider } from './features/websocket/WebSocketProvider';

const AuthStack = createNativeStackNavigator<AuthStackParamList>();
const Tab = createBottomTabNavigator<AppTabParamList>();

const navigationTheme = {
  ...DarkTheme,
  colors: {
    ...DarkTheme.colors,
    primary: colors.primary,
    background: colors.background,
    card: colors.surfaceStrong,
    text: colors.text,
    border: colors.border,
    notification: colors.danger
  }
};

function AuthNavigator() {
  return (
    <AuthStack.Navigator screenOptions={{ headerShown: false }}>
      <AuthStack.Screen name="Login" component={LoginScreen} />
    </AuthStack.Navigator>
  );
}

function iconForRoute(routeName: keyof AppTabParamList, color: string, size: number) {
  switch (routeName) {
    case 'Dashboard':
      return <LayoutDashboard color={color} size={size} />;
    case 'Budget':
      return <BarChart3 color={color} size={size} />;
    case 'Transactions':
      return <ReceiptText color={color} size={size} />;
    case 'Payees':
      return <Tags color={color} size={size} />;
    case 'Loans':
      return <Landmark color={color} size={size} />;
    case 'Settings':
      return <Settings color={color} size={size} />;
    default:
      return null;
  }
}

function TabIcon({ routeName, focused }: { routeName: keyof AppTabParamList; focused: boolean }) {
  const iconColor = focused ? colors.primary : colors.muted;

  return (
    <View style={[styles.tabIconPill, focused && styles.tabIconPillFocused]}>
      {iconForRoute(routeName, iconColor, focused ? 31 : 30)}
    </View>
  );
}

function AppTabs() {
  return (
    <>
      <WebSocketProvider />
      <Tab.Navigator
        initialRouteName="Dashboard"
        screenOptions={({ route }) => ({
          headerShown: false,
          tabBarShowLabel: false,
          tabBarActiveTintColor: colors.primary,
          tabBarInactiveTintColor: colors.muted,
          tabBarStyle: styles.tabBar,
          tabBarItemStyle: styles.tabBarItem,
          tabBarIconStyle: styles.tabBarIcon,
          tabBarIcon: ({ focused }) => <TabIcon routeName={route.name as keyof AppTabParamList} focused={focused} />
        })}
      >
        <Tab.Screen name="Dashboard" component={DashboardScreen} />
        <Tab.Screen name="Budget" component={BudgetScreen} />
        <Tab.Screen name="Transactions" component={TransactionsScreen} />
        <Tab.Screen name="Payees" component={PayeesScreen} />
        <Tab.Screen name="Loans" component={LoansScreen} />
        <Tab.Screen name="Settings" component={SettingsScreen} />
      </Tab.Navigator>
    </>
  );
}

const styles = StyleSheet.create({
  tabBar: {
    position: 'absolute',
    left: 18,
    right: 18,
    bottom: 18,
    height: 72,
    borderTopWidth: 0,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.borderMuted,
    borderRadius: 36,
    backgroundColor: colors.surfaceStrong,
    paddingHorizontal: 8,
    paddingTop: 8,
    paddingBottom: 8,
    elevation: 16,
    shadowColor: '#000000',
    shadowOffset: { width: 0, height: 10 },
    shadowOpacity: 0.25,
    shadowRadius: 18
  },
  tabBarItem: {
    height: 56,
    borderRadius: 28
  },
  tabBarIcon: {
    width: 60,
    height: 56
  },
  tabIconPill: {
    width: 56,
    height: 52,
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: 28
  },
  tabIconPillFocused: {
    backgroundColor: colors.surfaceTertiary
  }
});

function RootContent() {
  const dispatch = useAppDispatch();
  const auth = useAppSelector((state) => state.auth);
  const budgets = useAppSelector((state) => state.budgets);

  useEffect(() => {
    dispatch(hydrateAuth());
  }, [dispatch]);

  useEffect(() => {
    if (auth.hydrated && auth.isAuthenticated) {
      dispatch(fetchAllBudgets());
    }
  }, [auth.hydrated, auth.isAuthenticated, dispatch]);

  if (!auth.hydrated) {
    return <LoadingStateView label="Restoring session" />;
  }

  if (!auth.isAuthenticated) {
    return <AuthNavigator />;
  }

  if (budgets.loading === 'pending' && !budgets.selectedBudget) {
    return <LoadingStateView label="Loading budget" />;
  }

  if (!budgets.selectedBudget) {
    return <BudgetOnboardingScreen />;
  }

  return <AppTabs />;
}

function AppShell() {
  useEffect(() => {
    apiClient
      .probeRoot()
      .then(({ status, body, url }) => {
        console.log(`[api-probe] GET ${url} -> ${status}: ${body}`);
      })
      .catch((error: unknown) => {
        console.log('[api-probe] GET /api failed', error);
      });
  }, []);

  return (
    <SafeAreaProvider>
      <StatusBar style="light" />
      <NavigationContainer theme={navigationTheme}>
        <RootContent />
      </NavigationContainer>
    </SafeAreaProvider>
  );
}

export default function App() {
  return (
    <Provider store={store}>
      <AppShell />
    </Provider>
  );
}
