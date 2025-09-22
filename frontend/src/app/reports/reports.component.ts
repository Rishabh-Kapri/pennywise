import {
  AfterViewInit,
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  OnDestroy,
  TemplateRef,
  ViewChild,
  ViewContainerRef,
} from '@angular/core';
import { StoreService } from '../services/store.service';
import { Observable, Subject, combineLatest, combineLatestAll, filter, of, switchMap, take, takeUntil } from 'rxjs';
import { PopoverRef } from '../services/popover-ref';
import { PopoverService } from '../services/popover.service';
import { Store } from '@ngxs/store';
import { AccountsState } from '../store/dashboard/states/accounts/accounts.state';
import { CategoryGroupsState } from '../store/dashboard/states/categoryGroups/categoryGroups.state';
import { HelperService } from '../services/helper.service';
import * as Highcharts from 'highcharts';
import 'highcharts/modules/drilldown';
import { TransactionsState } from '../store/dashboard/states/transactions/transactions.state';
import { NormalizedTransaction, Transaction } from '../models/transaction.model';
import { CategoriesState } from '../store/dashboard/states/categories/categories.state';
import { INFLOW_CATEGORY_NAME } from '../constants/general';
import { PayeesState } from '../store/dashboard/states/payees/payees.state';
import { Amount, CategoryData, CategoryGroupReport, DateRange, IncomeData } from '../models/reports.model';
import { AccountGroups, CategoryGroups } from '../models/reports.model';

enum Tab {
  SPENDING = 'spending',
  NETWORTH = 'networth',
  INCOME_EXPENSE = 'incomeExpense',
}

@Component({
  selector: 'app-reports',
  templateUrl: './reports.component.html',
  styleUrl: './reports.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
  standalone: false,
})
export class ReportsComponent implements AfterViewInit, OnDestroy {
  @ViewChild('categoryContainer') categoryContainer: ElementRef;

  private activeFilterOverlayRef: PopoverRef;

  private readonly _destroy$ = new Subject<void>();

  tabEnum = Tab;

  showReports = false;
  currentTab: Tab = Tab.SPENDING;
  accountData$: Observable<AccountGroups[]>;
  categoryGroupData$: Observable<CategoryGroups[]>;

  budgetAccounts$ = this.ngxsStore.select(AccountsState.getBudgetAccounts);
  trackingAccounts$ = this.ngxsStore.select(AccountsState.getTrackingAccounts);

  categoryGroups$ = this.ngxsStore.select(CategoryGroupsState.getCategoryGroupData);

  // filter related
  categoryFilter: { text: string } = { text: 'All Categories' };
  accountFilter: { text: string } = { text: 'All Accounts' };
  selectedDateRange = { ...this.helperService.getCurrentMonthDateRange() };

  selectedCategories: any[] = [];
  dateRange: DateRange[] = [];

  // highcharts related
  // @TODO: show total inside the pie chart
  Highcharts: typeof Highcharts = Highcharts;
  chartConstructor: string = 'pie';
  chartOptions: Highcharts.Options = {
    title: {
      text: 'Spending',
      margin: 75,
      style: {
        color: 'white',
      },
    },
    chart: {
      type: 'pie',
      height: '45%',
      marginLeft: 200,
      backgroundColor: '#232325',
      style: {
        color: 'white',
      },
      events: {
        load: function () {},
        redraw: (e) => {},
        drilldown: (e) => {
          const series = e.seriesOptions as Highcharts.SeriesPieOptions;
          this.selectedCategories = series?.data as any[];
        },
      },
    },
    legend: {
      enabled: true,
      align: 'right',
      verticalAlign: 'middle',
      layout: 'vertical',
      x: -200,
      y: 50,
      itemMarginBottom: 20,
      // labelFormat: '{name}: ₹{y}',
      labelFormatter: function () {
        const options = this.options as any;
        return `${this.name.substring(0, 10)}: ₹${options.y}`;
        // return `${this.name}: ${options.y}`
      },
      itemStyle: {
        color: '#d4d8d4',
        fontSize: '0.85rem',
      },
      itemHoverStyle: {
        color: '#ffffff',
      },
    },
    tooltip: {
      headerFormat: '<span style="color:{point.color};font-size:12px;"><b>{point.name}</b></span><br>',
      pointFormat:
        '<span style="font-size:14px">₹{point.y:.2f}</span><br>' + '<b>{point.percentage:.2f}%</b> of total<br/>',
    },
    plotOptions: {
      pie: {
        innerSize: '60%',
        depth: 45,
        dataLabels: [
          {
            enabled: true,
            distance: 15,
            style: {
              color: 'white',
            },
            formatter: function () {
              return this.name.substring(0, 10);
              // return this.name;
            },
          },
          {
            enabled: true,
            distance: '-20%',
            filter: {
              property: 'percentage',
              operator: '>',
              value: 4,
            },
            format: '{point.percentage:.1f}%',
            style: {
              fontSize: '0.9rem',
              textOutline: 'none',
            },
          },
        ],
      },
      series: {
        allowPointSelect: true,
        cursor: 'pointer',
        showInLegend: true,
      },
    },
    series: [
      {
        name: 'All Categories',
      },
    ] as Highcharts.SeriesPieOptions[],
    drilldown: {
      breadcrumbs: {
        format: '{level.name}',
        position: {
          x: -100,
          y: -100,
        },
        style: {
          style: {
            color: 'white',
          },
        },
        buttonTheme: {
          style: {
            color: 'rgb(44,175,254)',
          },
        },
        separator: {
          text: '▶',
          style: {
            color: '#ffffff',
          },
        },
      },
      series: [],
    } as Highcharts.DrilldownOptions,
  };
  chartCallback: Highcharts.ChartCallbackFunction = function (chart) {};
  updateFlag = false;
  oneToOneFlag = false;
  runOutsideAngular = false;

  isFilterApplied = false;
  incomeData: IncomeData[] = [];
  categoriesExpenseData: CategoryGroupReport[] = [];
    
  constructor(
    private ngxsStore: Store,
    private cdr: ChangeDetectorRef,
    private popper: PopoverService,
    private viewContainerRef: ViewContainerRef,
    private helperService: HelperService,
    public store: StoreService,
  ) {
    this.accountData$ = combineLatest([this.budgetAccounts$, this.trackingAccounts$]).pipe(
      takeUntil(this._destroy$),
      switchMap(([budgetAccounts, trackingAccounts]) => {
        const groupData = [
          {
            name: 'Budget Accounts',
            isChecked: true,
            accounts: [
              ...budgetAccounts.map((account) => {
                return {
                  ...account,
                  isChecked: true,
                };
              }),
            ],
          },
          {
            name: 'Tracking Accounts',
            isChecked: true,
            accounts: trackingAccounts.map((account) => {
              return {
                ...account,
                isChecked: true,
              };
            }),
          },
        ];
        return of(groupData);
      }),
    );
    this.categoryGroupData$ = combineLatest([this.categoryGroups$]).pipe(
      takeUntil(this._destroy$),
      switchMap(([categoryGroupData]) => {
        const groups: CategoryGroups[] = [];
        for (const groupData of categoryGroupData) {
          let isChecked = false;
          if (groupData.name === 'Investments') {
            isChecked = true;
          }
          // @TODO: put this in a constant
          if (groupData.name !== 'Credit Card Payments' && groupData.name !== 'Hidden') {
            groups.push({
              ...groupData,
              isChecked: true,
              categories: [
                ...groupData.categories.map((cat) => {
                  return {
                    ...cat,
                    isChecked: true,
                  };
                }),
              ],
            });
          }
        }
        return of(groups);
      }),
    );
    combineLatest([this.categoryGroupData$, this.accountData$])
      .pipe(
        filter(([categoryGroups, accountGroups]) => categoryGroups.length > 0 && accountGroups[1]?.accounts.length > 0),
      )
      .subscribe(([categoryGroups, accountGroups]) => {
        if (!this.isFilterApplied) {
          this.applyFilter(categoryGroups, accountGroups);
        }
      });
  }

  ngAfterViewInit(): void {}

  changeTab(tab: Tab) {
    this.currentTab = tab;
    switch (this.currentTab) {
      case Tab.SPENDING: {
        combineLatest([this.categoryGroupData$, this.accountData$])
          .pipe(
            filter(
              ([categoryGroups, accountGroups]) => categoryGroups.length > 0 && accountGroups[1]?.accounts.length > 0,
            ),
            take(1),
          )
          .subscribe(([categoryGroups, accountGroups]) => {
            if (!this.isFilterApplied) {
              this.applyFilter(categoryGroups, accountGroups);
            }
          });
        break;
      }
      case Tab.NETWORTH: {
        break;
      }
      case Tab.INCOME_EXPENSE: {
        combineLatest([this.categoryGroupData$, this.accountData$])
          .pipe(
            filter(
              ([categoryGroups, accountGroups]) => categoryGroups.length > 0 && accountGroups[1]?.accounts.length > 0,
            ),
            take(1),
          )
          .subscribe(([categoryGroups, accountGroups]) => {
            console.log('categoryGroups:::', categoryGroups);
            if (!this.isFilterApplied) {
              // const startDate = this.helperService.getDateInStringFormat(new Date(), -3);
              // const endDate = this.helperService.getDateInStringFormat(new Date(), 1);
              // this.selectedDateRange = {
              //   startDate,
              //   endDate,
              // };
              this.dateRange = this.helperService.getDateRange(this.selectedDateRange.startDate, this.selectedDateRange.endDate);
              this.getIncomeSources();
              this.getCategoriesExpense(categoryGroups);
              this.applyFilter(categoryGroups, accountGroups);
            }
          });
        break;
      }
    }
  }

  showFilterPopover(content: TemplateRef<any>, origin: HTMLElement) {
    if (this.activeFilterOverlayRef?.isOpen) {
      this.activeFilterOverlayRef.close();
    }
    this.activeFilterOverlayRef = this.popper.open({ origin, content, viewContainerRef: this.viewContainerRef });
  }

  selectFilter(
    filter: 'categories' | 'accounts',
    type: 'all' | 'none' | 'group' | 'item',
    allGroups: any,
    group?: any,
    item?: any,
  ) {
    const filterObj = filter === 'categories' ? this.categoryFilter : this.accountFilter;
    switch (type) {
      case 'all': {
        filterObj.text = `All ${filter}`;
        allGroups.forEach((group: any) => {
          group.isChecked = true;
          group[filter].forEach((item: any) => {
            item.isChecked = true;
          });
        });
        break;
      }
      case 'none': {
        filterObj.text = `Select ${filter}`;
        allGroups.forEach((group: any) => {
          group.isChecked = false;
          group[filter].forEach((item: any) => {
            item.isChecked = false;
          });
        });
        break;
      }
      case 'group': {
        filterObj.text = `Some ${filter}`;
        if (group) {
          group.isChecked = !group.isChecked;
          group[filter].forEach((item: any) => {
            item.isChecked = group.isChecked;
          });
        }
        break;
      }
      case 'item': {
        filterObj.text = `Some ${filter}`;
        if (item) {
          item.isChecked = !item.isChecked;
        }
        if (group) {
          group.isChecked = group[filter].some((item: any) => item.isChecked);
        }
        break;
      }
    }
    if (filter === 'categories') {
      this.categoryGroupData$ = of(allGroups);
    } else if (filter === 'accounts') {
      this.accountData$ = of(allGroups);
    }
  }

  selectDate(event: any, key: 'startDate' | 'endDate') {
    const filteredDate = event.target.value
      .split('-')
      .map((val: string) => val.replace(/^0+/, ''))
      .join('-');
    this.selectedDateRange[key] = filteredDate;
  }

  getSelectedCategories(categoryGroups: CategoryGroups[]): string[] {
    const categoryGroupMap = categoryGroups
      .map((group) => {
        const checkedCategories = group.categories
          .filter((cat) => cat.isChecked)
          .map((cat) => ({ id: cat.id!, name: cat.name }));
        return checkedCategories.length > 0 ? [group.id!, { name: group.name, categories: checkedCategories }] : null;
      })
      .filter(Boolean);
    const categoryIds = categoryGroups.flatMap((group) =>
      group.categories.filter((cat) => cat.isChecked).map((cat) => cat.id!),
    );
    return categoryIds;
  }

  applyFilter(categoryGroups: CategoryGroups[], accountGroups: AccountGroups[]) {
    this.isFilterApplied = true;
    if (this.activeFilterOverlayRef?.isOpen) {
      this.activeFilterOverlayRef.close();
    }

    const allTransactions = this.ngxsStore.selectSnapshot(TransactionsState.getNormalizedTransaction);
    const accountIds = accountGroups.flatMap((group: any) =>
      group.accounts.filter((acc: any) => acc.isChecked).map((acc: any) => acc.id),
    );

    const { startDate, endDate } = this.selectedDateRange;
    // get transaction between date range for the categories and accounts selected
    const filteredTransactions = this.helperService.filterTransactionsReport(
      allTransactions,
      this.getSelectedCategories(categoryGroups),
      accountIds,
      startDate,
      endDate,
    );

    let chartSeries = [];
    const drilldownSeries = [];
    for (const [groupId, groupValue] of categoryGroups.entries()) {
      const transactionAmount = this.getTransactionsAmount(
        <Transaction[]>filteredTransactions,
        groupValue.categories.map((cat: any) => cat.id),
      );
      if (transactionAmount) {
        chartSeries.push({
          name: groupValue.name,
          y: Number(transactionAmount.toFixed(2)),
          drilldown: groupValue.name,
        });
        drilldownSeries.push({
          name: groupValue.name,
          id: groupValue.name,
          data: this.getCategoriesData(<Transaction[]>filteredTransactions, groupValue.categories),
        });
      }
    }
    if (this.chartOptions.series?.[0]) {
      this.chartOptions.series[0] = {
        name: 'All Categories',
        innerSize: '60%',
        data: chartSeries,
      } as Highcharts.SeriesPieOptions;
    }
    if (this.chartOptions.drilldown?.series) {
      this.chartOptions.drilldown.series = drilldownSeries as Array<Highcharts.SeriesOptionsType>;
    }
    this.showReports = true;
    this.updateFlag = true;
    this.selectedCategories = chartSeries;

    this.cdr.markForCheck();
    this.isFilterApplied = false;
  }

  getCategoriesData(transactions: Transaction[], categoryData: any[]) {
    let data = [];
    for (const category of categoryData) {
      const amount = this.getTransactionsAmount(transactions, [category.id]);
      if (amount) {
        data.push([category.name, Number(amount.toFixed(2))]);
      }
    }
    return data;
  }

  getTransactionsAmount(transactions: Transaction[], categoryIds: string[]) {
    const amount = transactions
      .filter((txn) => categoryIds.includes(txn.categoryId ?? ''))
      .reduce((acc, curr) => acc + curr.amount, 0);
    return Math.abs(amount);
  }

  getIncomeSources() {
    this.incomeData = [];
    const incomeSources: { [payeeId: string]: { [monthKey: string]: number } } = {};
    const allTransactions = this.ngxsStore.selectSnapshot(TransactionsState.getNormalizedTransaction);
    // get all income transactions for the month range
    const dateMap = new Map();

    const incomeCategory = this.ngxsStore.selectSnapshot(CategoriesState.getCategoryFromName(INFLOW_CATEGORY_NAME));
    for (const month of this.dateRange) {
      if (incomeCategory) {
        dateMap.set(month.monthKey, {});
        const monthTxns = this.helperService.filterTransactionsBasedOnMonth(allTransactions, month.monthKey);
        const incomeTxns = this.helperService.getTransactionsForCategory(monthTxns, [incomeCategory.id!]);
        const payeeMap = new Map();
        for (const incomeTxn of incomeTxns) {
          const payees = dateMap.get(month.monthKey);
          payees[incomeTxn.payeeId] = incomeTxn.amount + (payees[incomeTxn.payeeId] ?? 0);
        }

        for (const [payeeId, amount] of payeeMap) {
          incomeSources[payeeId] = {
            ...incomeSources[payeeId],
            [month.monthKey]: amount,
          };
        }
      }
    }
    for (const [monthKey, payees] of dateMap) {
      for (const [payeeId, amount] of Object.entries(payees)) {
        // const foundPayee = this.ngxsStore.selectSnapshot(PayeesState.getPayeeFromId(payeeId));
        if (incomeSources[payeeId]) {
          incomeSources[payeeId][monthKey] = amount as number;
        } else {
          incomeSources[payeeId] = {
            [monthKey]: amount as number,
          };
        }
      }
    }
    for (const [payeeId, value] of Object.entries(incomeSources)) {
      const foundPayee = this.ngxsStore.selectSnapshot(PayeesState.getPayeeFromId(payeeId));
      this.incomeData.push({
        payee: foundPayee?.name ?? '',
        amounts: value,
      });
    }
  }

  getCategoriesExpense(categoryGroups: CategoryGroups[]) {
    const allTransactions = this.ngxsStore.selectSnapshot(TransactionsState.getNormalizedTransaction);

    // Pre-compute monthly transactions
    const monthlyTxnsCache = new Map<string, NormalizedTransaction[]>();
    for (const month of this.dateRange) {
      monthlyTxnsCache.set(
        month.monthKey,
        this.helperService.filterTransactionsBasedOnMonth(allTransactions, month.monthKey),
      );
    }
    this.categoriesExpenseData = categoryGroups
      .filter((group) => group.isChecked && group.categories.some((cat) => cat.isChecked))
      .map((group) => {
        console.log(group);
        const categories = group.categories
          .filter((cat) => cat.isChecked)
          .map((cat) => {
            console.log(cat);
            const amounts: Amount = {};
            for (const month of this.dateRange) {
              const monthTxns = monthlyTxnsCache.get(month.monthKey)!;
              const categoryTxns = this.helperService.getTransactionsForCategory(monthTxns, [cat.id!]);
              console.log(month, categoryTxns);
              // amounts[month.monthKey] = Math.abs(this.helperService.sumTransaction(categoryTxns));
              amounts[month.monthKey] = (this.helperService.sumTransaction(categoryTxns));
            }
            return {
              name: cat.name,
              amounts,
            };
          });
        return {
          groupName: group.name,
          collapse: false,
          categories,
        };
      });
  }

  ngOnDestroy(): void {
    this._destroy$.next();
    this._destroy$.complete();
    if (this.activeFilterOverlayRef?.isOpen) {
      this.activeFilterOverlayRef.close();
    }
  }
}
