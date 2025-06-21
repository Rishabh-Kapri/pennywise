import { NgModule } from '@angular/core';
import { BrowserModule } from '@angular/platform-browser';
import { CommonModule } from '@angular/common';
import { OverlayModule } from '@angular/cdk/overlay';
import { AppRoutingModule } from './app-routing.module';
import { AppComponent } from './app.component';
import { SidebarComponent } from './sidebar/sidebar.component';
import { HeaderComponent } from './header/header.component';
import { TransactionsComponent } from './transactions/transactions.component';
import { BudgetComponent } from './budget/budget.component';
import { NgIconsModule } from '@ng-icons/core';
import {
  heroHome,
  heroRectangleStack,
  heroBuildingLibrary,
  heroBanknotes,
  heroCurrencyRupee,
  heroChevronDown,
  heroChevronRight,
  heroPlusCircle,
  heroXMark,
  heroChevronLeft,
  heroMagnifyingGlass,
} from '@ng-icons/heroicons/outline';
import { heroPencilSolid, heroPlusCircleSolid, heroCheckCircleSolid } from '@ng-icons/heroicons/solid';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { initializeApp, provideFirebaseApp } from '@angular/fire/app';
import { environment } from 'src/environment/environment';
import { getFirestore, provideFirestore } from '@angular/fire/firestore';
import { DashboardComponent } from './dashboard/dashboard.component';
import { DatePickerComponent } from './budget/date-picker/date-picker.component';
import { CategoryComponent } from './budget/category/category.component';
import { CategoryItemComponent } from './budget/category-item/category-item.component';
import { ReportsComponent } from './reports/reports.component';
import { NgxsStoreModule } from './store/store.module';
import { AutoFocusDirective } from './directives/autofocus.directive';
import { AbsolutePipe } from './pipes/absolute.pipe';
import { HighchartsChartModule } from 'highcharts-angular';
import { CalculateAveragePipe } from './pipes/calculateAverage.pipe';
import { CalculateTotalPipe } from './pipes/calculateTotal.pipe';
import { AddZeroPrefixToDate } from './pipes/addZeroPrefixDate.pipe';
import { AccountsMobileComponent } from './accounts/accounts-mobile.component';
import { TransactionsMobileComponent } from './transactions/mobile/transactions-mobile.component';

@NgModule({
  declarations: [
    AppComponent,
    SidebarComponent,
    HeaderComponent,
    BudgetComponent,
    TransactionsComponent,
    DashboardComponent,
    ReportsComponent,
    DatePickerComponent,
    CategoryComponent,
    CategoryItemComponent,
    AutoFocusDirective,
    AbsolutePipe,
    CalculateAveragePipe,
    CalculateTotalPipe,
    AddZeroPrefixToDate,
    AccountsMobileComponent,
    TransactionsMobileComponent,
  ],
  imports: [
    BrowserModule,
    AppRoutingModule,
    CommonModule,
    FormsModule,
    ReactiveFormsModule,
    OverlayModule,
    provideFirebaseApp(() => initializeApp(environment.firebase)),
    provideFirestore(() => getFirestore()),
    NgIconsModule.withIcons({
      heroHome,
      heroRectangleStack,
      heroBuildingLibrary,
      heroBanknotes,
      heroCurrencyRupee,
      heroChevronDown,
      heroPencilSolid,
      heroPlusCircle,
      heroXMark,
      heroChevronLeft,
      heroChevronRight,
      heroPlusCircleSolid,
      heroCheckCircleSolid,
      heroMagnifyingGlass,
    }),
    NgxsStoreModule,
    HighchartsChartModule,
  ],
  providers: [],
  bootstrap: [AppComponent],
})
export class AppModule {}
