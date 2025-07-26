import { AfterContentInit, Directive, ElementRef, Input } from '@angular/core';

@Directive({
  selector: '[autoFocus]',
  standalone: false,
})
export class AutoFocusDirective implements AfterContentInit {
  @Input() focus: boolean;

  constructor(private el: ElementRef) {}

  ngAfterContentInit(): void {
    setTimeout(() => {
      this.el.nativeElement.focus();
    }, 500);
  }
}
