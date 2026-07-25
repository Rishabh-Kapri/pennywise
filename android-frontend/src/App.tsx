import 'react-native-gesture-handler';
import { useEffect } from 'react';
import { StyleSheet, View } from 'react-native';
import { Provider } from 'react-redux';
import { DarkTheme, NavigationContainer } from '@react-navigation/native';
import { createBottomTabNavigator } from '@react-navigation/bottom-tabs';
import { createNativeStackNavigator } from '@react-navigation/native-stack';
import { SafeAreaProvider, useSafeAreaInsets } from 'react-native-safe-area-context';
import { StatusBar } from 'expo-status-bar';
import { BarChart3, Landmark, LayoutDashboard, ReceiptText, Sparkles, Tags } from 'lucide-react-native';
import { useAppDispatch, useAppSelector } from './app/hooks';
import { store } from './app/store';
import { LoadingStateView } from './components/LoadingStateView';
import { colors, spacing } from './theme';
import { apiClient } from './utils/api';
import type { AppTabParamList, AuthStackParamList, RootStackParamList } from './navigation/types';
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
import { AgentChat } from './features/agent/components/AgentChat';

const AuthStack = createNativeStackNavigator<AuthStackParamList>();
const RootStack = createNativeStackNavigator<RootStackParamList>();
const Tab = createBottomTabNavigator<AppTabParamList>();

const navigationTheme = {
  ...DarkTheme,
  colors: {
    ...DarkTheme.colors,
    primary: colors.primary,
    background: colors.background,
    card: colors.surface,
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
    case 'Penny':
      return <Sparkles color={color} size={size} />;
    default:
      return null;
  }
}

function TabIcon({ routeName, focused }: { routeName: keyof AppTabParamList; focused: boolean }) {
  const iconColor = focused ? colors.primary : colors.faint;

  return (
    <View style={[styles.tabIconPill, focused && styles.tabIconPillFocused]}>
      {iconForRoute(routeName, iconColor, 22)}
    </View>
  );
}

function AppTabs() {
  const insets = useSafeAreaInsets();

  return (
    <Tab.Navigator
      initialRouteName="Dashboard"
      screenOptions={({ route }) => ({
        headerShown: false,
        tabBarShowLabel: false,
        tabBarHideOnKeyboard: true,
        tabBarActiveTintColor: colors.primary,
        tabBarInactiveTintColor: colors.muted,
        tabBarStyle: [styles.tabBar, { bottom: Math.max(insets.bottom, spacing.md) }],
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
      <Tab.Screen name="Penny" component={AgentChat} />
    </Tab.Navigator>
  );
}

function AppNavigator() {
  return (
    <>
      <WebSocketProvider />
      <RootStack.Navigator screenOptions={{ headerShown: false }}>
        <RootStack.Screen name="Main" component={AppTabs} />
        <RootStack.Screen name="Settings" component={SettingsScreen} />
      </RootStack.Navigator>
    </>
  );
}

const styles = StyleSheet.create({
  tabBar: {
    position: 'absolute',
    // React Navigation's base tab bar style pins the bar with the logical
    // `start`/`end` insets, which win over `left`/`right` in Yoga no matter the
    // style order. Override the same properties so the bar actually floats.
    start: spacing.lg,
    end: spacing.lg,
    height: 64,
    borderTopWidth: 0,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.borderLight,
    borderRadius: 32,
    backgroundColor: colors.surface,
    paddingHorizontal: spacing.xs,
    paddingTop: spacing.sm,
    paddingBottom: spacing.sm,
    elevation: 24,
    shadowColor: '#000000',
    shadowOffset: { width: 0, height: 10 },
    shadowOpacity: 0.5,
    shadowRadius: 20
  },
  tabBarItem: {
    height: 48,
    borderRadius: 24
  },
  tabBarIcon: {
    height: 48
  },
  tabIconPill: {
    width: 44,
    height: 44,
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: 22
  },
  tabIconPillFocused: {
    backgroundColor: colors.primaryMuted
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

  return <AppNavigator />;
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
