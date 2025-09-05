import { Component, OnInit, TemplateRef, ViewContainerRef } from '@angular/core';
import { Store } from '@ngxs/store';
import { HelperService } from 'src/app/services/helper.service';
import { PopoverRef } from 'src/app/services/popover-ref';
import { PopoverService } from 'src/app/services/popover.service';
import { StoreService } from 'src/app/services/store.service';
import { BudgetsActions } from 'src/app/store/dashboard/states/budget/budget.action';

enum DATE_VALUE {
  MONTH,
  YEAR,
}
@Component({
  selector: 'app-date-picker',
  templateUrl: './date-picker.component.html',
  styleUrls: ['./date-picker.component.scss'],
  standalone: false,
})
export class DatePickerComponent implements OnInit {
  dateValue = DATE_VALUE;
  MONTH_NAMES = [
    { value: 'January', shortName: 'Jan', type: DATE_VALUE.MONTH },
    { value: 'February', shortName: 'Feb', type: DATE_VALUE.MONTH },
    { value: 'March', shortName: 'Mar', type: DATE_VALUE.MONTH },
    { value: 'April', shortName: 'Apr', type: DATE_VALUE.MONTH },
    { value: 'May', shortName: 'May', type: DATE_VALUE.MONTH },
    { value: 'June', shortName: 'Jun', type: DATE_VALUE.MONTH },
    { value: 'July', shortName: 'Jul', type: DATE_VALUE.MONTH },
    { value: 'August', shortName: 'Aug', type: DATE_VALUE.MONTH },
    { value: 'September', shortName: 'Sep', type: DATE_VALUE.MONTH },
    { value: 'October', shortName: 'Oct', type: DATE_VALUE.MONTH },
    { value: 'Novemeber', shortName: 'Nov', type: DATE_VALUE.MONTH },
    { value: 'December', shortName: 'Dec', type: DATE_VALUE.MONTH },
  ];
  YEARS = [
    { value: '2030', shortName: '2030', type: DATE_VALUE.YEAR },
    { value: '2029', shortName: '2029', type: DATE_VALUE.YEAR },
    { value: '2028', shortName: '2028', type: DATE_VALUE.YEAR },
    { value: '2027', shortName: '2027', type: DATE_VALUE.YEAR },
    { value: '2026', shortName: '2026', type: DATE_VALUE.YEAR },
    { value: '2025', shortName: '2025', type: DATE_VALUE.YEAR },
    { value: '2024', shortName: '2024', type: DATE_VALUE.YEAR },
    { value: '2023', shortName: '2023', type: DATE_VALUE.YEAR },
    { value: '2022', shortName: '2022', type: DATE_VALUE.YEAR },
    { value: '2021', shortName: '2021', type: DATE_VALUE.YEAR },
  ];
  dateValues: any[] = [];
  showDatepicker = false;
  datepickerValue: string;
  selectedMonth: number;
  selectedYear: number;
  showTodayButton = false;
  dateOverlayRef: PopoverRef;

  constructor(
    private ngxsStore: Store,
    public store: StoreService,
    private viewContainerRef: ViewContainerRef,
    private popper: PopoverService,
    private helperService: HelperService,
  ) {}

  ngOnInit(): void {
    this.initDate();
  }

  setSelectedMonthKey() {
    const monthKey = this.helperService.getSelectedMonthInHumanFormat(`${this.selectedYear}-${this.selectedMonth}`);
    this.ngxsStore.dispatch(new BudgetsActions.SetSelectedMonth(monthKey));
  }

  initDate() {
    const today = new Date();
    this.selectedMonth = today.getMonth();
    this.selectedYear = today.getFullYear();
    this.setSelectedMonthKey();
  }

  showDateSelect(content: TemplateRef<any>, origin: any) {
    if (this.dateOverlayRef?.isOpen) {
      return;
    }
    this.dateValues = this.MONTH_NAMES;
    this.dateOverlayRef = this.popper.open({ origin, content, viewContainerRef: this.viewContainerRef });
  }

  changeYear() {
    this.dateValues = this.YEARS;
  }

  selectValue(index: number, obj: typeof this.MONTH_NAMES) {
    const value = obj[index];
    if (value.type === DATE_VALUE.YEAR) {
      this.selectedYear = Number(value.value);
      this.isToday();
      this.setSelectedMonthKey();
    } else if (value.type === DATE_VALUE.MONTH) {
      this.selectMonth(index);
    }
    this.dateValues = this.MONTH_NAMES;
  }

  isToday() {
    const today = new Date();
    const selectedDate = new Date(today);
    selectedDate.setMonth(this.selectedMonth);
    selectedDate.setFullYear(this.selectedYear);
    this.showTodayButton = !(today.toDateString() === selectedDate.toDateString());
  }

  selectYear(add: number) {
    this.selectedYear += add;
    this.isToday();
    this.setSelectedMonthKey();
  }

  changeMonth(add: number) {
    const newMonth = this.selectedMonth + add;
    const newDate = new Date(this.selectedYear, newMonth);
    this.selectedYear = newDate.getFullYear();
    this.selectedMonth = newDate.getMonth();
    this.isToday();
    this.setSelectedMonthKey();
  }

  selectMonth(month: number) {
    this.selectedMonth = month;
    this.isToday();
    this.setSelectedMonthKey();
  }

  selectToday() {
    const date = new Date();
    this.selectedMonth = date.getMonth();
    this.selectedYear = date.getFullYear();
    this.isToday();
    this.setSelectedMonthKey();
  }
}
