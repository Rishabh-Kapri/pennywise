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
  heroPlusCircle,
  heroXMark,
  heroChevronLeft,
  heroChevronRight,
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

@NgModule({
  declarations: [
    AppComponent,
    SidebarComponent,
    HeaderComponent,
    BudgetComponent,
    TransactionsComponent,
    DashboardComponent,
    DatePickerComponent,
    CategoryComponent,
    CategoryItemComponent,
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
    ReportsComponent,
    NgxsStoreModule,
  ],
  providers: [],
  bootstrap: [AppComponent],
})
export class AppModule {}
