import 'react';
import type { PrintDesignerElement } from '@go-wind/print-common';

/**
 * @worm-vue3-print/common 的 <print-designer> 自定义元素（Custom Element + Shadow DOM）
 * 的 JSX 类型补充。对象入参（fields/template/fonts/uploadImage）必须走 ref 赋 property
 * （JSX 属性会 setAttribute 字符串化），这里只声明字符串开关属性。
 */
declare module 'react' {
  namespace JSX {
    interface IntrinsicElements {
      'print-designer': React.DetailedHTMLProps<
        React.HTMLAttributes<PrintDesignerElement>,
        PrintDesignerElement
      > & {
        'is-edit'?: string | boolean;
        'show-help'?: string | boolean;
      };
    }
  }
}
