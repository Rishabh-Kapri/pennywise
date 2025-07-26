import { Pipe, PipeTransform } from '@angular/core';
import { Amount } from '../models/reports.model';

@Pipe({
  name: 'calculateAverage',
  standalone: false,
  pure: true,
})
export class CalculateAveragePipe implements PipeTransform {
  transform(amounts: Amount, months: number) {
    let total = 0;
    for (const amount of Object.values(amounts)) {
      total += amount;
    }
    return total / months;
  }
}
